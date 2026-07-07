import dataclasses
import datasets
from trl import SFTTrainer, SFTConfig
from transformers import HfArgumentParser
from transformers import TrainerCallback
import json
import math
import os
import csv
import torch
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    HfArgumentParser,
    TrainerCallback,
    BitsAndBytesConfig
)

from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training


@dataclasses.dataclass
class ScriptArguments:
    model_id: str
    dataset_path: str
    eval_dataset_path: str


class MetaflowBridge(TrainerCallback):

    def __init__(self, batch_size: int, rank: int, filename="/tmp/results/training_log.csv"):
        self.filename = filename.replace(".csv", f"_{rank}.csv")
        self.batch_size = batch_size

        directory = os.path.dirname(self.filename)

        if directory and not os.path.exists(directory):
            os.makedirs(directory, exist_ok=True)

        if not os.path.exists(self.filename):
            with open(self.filename, 'w', newline='') as f:
                writer = csv.writer(f)
                writer.writerow(["step", "loss", "learning_rate", "processed_data"])

    def on_log(self, args, state, control, logs=None, **kwargs):
        print("Captured log", logs)
        if logs and "loss" in logs:
            with open(self.filename, 'a', newline='') as f:
                writer = csv.writer(f)
                writer.writerow([
                    state.global_step, 
                    logs["loss"], 
                    logs.get("learning_rate", 0),
                    state.global_step * self.batch_size,
                ])



def _load_prompt_completion(path: str) -> datasets.Dataset:
    # Drop `text`: with completion_only_loss=True the SFTTrainer collator routes
    # any dataset containing `text` to language-modeling mode and raises. Keep
    # only prompt/completion so the loss is masked to the assistant turn.
    ds = datasets.load_from_disk(path)
    if "text" in ds.column_names:
        ds = ds.select_columns(["prompt", "completion"])
    return ds


def train_model(script_args: ScriptArguments, training_args: SFTConfig) -> None:
    dataset = _load_prompt_completion(script_args.dataset_path)
    print(f"Train dataset found with {dataset.features} features")

    eval_dataset = _load_prompt_completion(script_args.eval_dataset_path)
    print(f"Eval dataset found with {len(eval_dataset)} examples")
    print(training_args)


    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.bfloat16,
        bnb_4bit_use_double_quant=True,
    )
    local_rank = int(os.environ.get("LOCAL_RANK", 0))
    print("Env variables", os.environ)
    device_map = {"": local_rank}

    # No trust_remote_code: use transformers' NATIVE deepseek_v2. The remote
    # modeling code calls torch.utils.checkpoint without threading use_reentrant,
    # so it ignores our non-reentrant setting; the native model honors the
    # gradient_checkpointing_kwargs below, which is required for DDP (MoE).
    model = AutoModelForCausalLM.from_pretrained(
        script_args.model_id,
        quantization_config=bnb_config,
        device_map=device_map,
    )
    tokenizer = AutoTokenizer.from_pretrained(script_args.model_id)
    # Gradient checkpointing OFF: DeepSeek-V2 calls torch.utils.checkpoint
    # directly (ignoring use_reentrant), so the reentrant variant is unavoidable
    # and breaks DDP with "marked ready twice". Disabling checkpointing removes
    # the reentrant backward entirely; ddp_find_unused_parameters=True then
    # handles the MoE routing. Trade-off: higher activation memory (may OOM on a
    # small GPU).
    model = prepare_model_for_kbit_training(model, use_gradient_checkpointing=False)

    lora_config = LoraConfig(
        r=16,
        lora_alpha=16,
        target_modules=[
            "q_proj", 
            "kv_a_proj_with_mqa", 
            "kv_b_proj", 
            "o_proj",
            
            # --- Módulos del MLP / Expertos (MoE) ---
            "gate_proj", 
            "up_proj", 
            "down_proj"
        ],
        lora_dropout=0.05,
        bias="none",
        task_type="CAUSAL_LM"
    )
    model = get_peft_model(model, lora_config)

    print("Model loaded")

    trainer = SFTTrainer(
        model=model,
        processing_class=tokenizer,
        train_dataset=dataset,
        eval_dataset=eval_dataset,
        args=training_args,
        callbacks=[MetaflowBridge(batch_size=training_args.per_device_train_batch_size, rank=os.environ.get("RANK"))],
    )
    print("Trainer created")

    # SFTTrainer.__init__ re-freezes all params; restore LoRA trainability after it
    for name, param in model.named_parameters():
        if "lora_" in name:
            param.requires_grad_(True)

    train_dataloader = trainer.get_train_dataloader()
    num_batches = len(train_dataloader)
    batch_size = training_args.per_device_train_batch_size

    print(f"--- NODE {os.environ.get('RANK')} CHECK ---")
    print(f"Total batches on this node: {num_batches}")
    print(f"Total records on this node (approx): {num_batches * batch_size}")


    checkpoint_exists = False
    if os.path.exists(training_args.output_dir):
        checkpoints = [d for d in os.listdir(training_args.output_dir) if "checkpoint" in d]
        checkpoint_exists = len(checkpoints) > 0

    print("Training started")
    model.print_trainable_parameters()
    trainer.train(resume_from_checkpoint=checkpoint_exists)

    # Final assistant-only perplexity, computed ONCE after training (no periodic
    # eval, so it adds no per-step overhead). perplexity = exp(eval_loss).
    print("Computing final perplexity on the eval split...")
    metrics = trainer.evaluate()
    eval_loss = metrics["eval_loss"]
    perplexity = math.exp(eval_loss)
    print(f"FINAL  eval_loss={eval_loss:.6f}  perplexity={perplexity:.6f}")

    if trainer.is_world_process_zero():
        os.makedirs("/tmp/results", exist_ok=True)
        result = {
            "model_id": script_args.model_id,
            "eval_dataset_path": script_args.eval_dataset_path,
            "eval_loss": eval_loss,
            "perplexity": perplexity,
            "num_sequences": len(eval_dataset),
            "max_length": training_args.max_length,
        }
        with open("/tmp/results/perplexity.json", "w") as f:
            json.dump(result, f, indent=2)


def main():
    parser = HfArgumentParser([ScriptArguments, SFTConfig])
    script_args, training_args = parser.parse_args_into_dataclasses()
    train_model(script_args, training_args)


if __name__ == "__main__":
    main()