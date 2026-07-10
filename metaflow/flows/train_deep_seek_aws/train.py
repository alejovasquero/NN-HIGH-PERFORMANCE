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
        # print("Captured log", logs)
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


    print("Env variables", os.environ)

    # QLoRA: base en 4-bit (NF4 + double quant), cómputo en bf16. TODO el pipeline
    # (local, multinodo y perplexity base) va en 4-bit para que:
    #  - la comparación de TIEMPOS local vs multinodo no confunda precisión con
    #    paralelismo (misma precisión en ambos), y
    #  - la perplexity base vs fine-tuned sea comparable (ambas en 4-bit).
    # El hang que antes se achacó a Params4bit era el pipe deadlock de NCCL_DEBUG
    # (resuelto). device_map fija el modelo en la GPU local de cada rank porque
    # bitsandbytes cuantiza en GPU al cargar (ZeRO-2, 1 GPU/nodo -> LOCAL_RANK=0).
    local_rank = int(os.environ.get("LOCAL_RANK", 0))
    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_use_double_quant=True,
        bnb_4bit_compute_dtype=torch.bfloat16,
    )
    model = AutoModelForCausalLM.from_pretrained(
        script_args.model_id,
        quantization_config=bnb_config,
        device_map={"": local_rank},
        torch_dtype=torch.bfloat16,
        trust_remote_code=True,
    )
    tokenizer = AutoTokenizer.from_pretrained(script_args.model_id)
    # Prepara el modelo 4-bit para entrenar: castea layernorms a fp32, habilita
    # input grads y gradient checkpointing para que los grads lleguen a los
    # adapters LoRA.
    model = prepare_model_for_kbit_training(model, use_gradient_checkpointing=True)

    lora_config = LoraConfig(
        r=16,
        lora_alpha=16,
        target_modules=[
            "q_proj",
            "kv_a_proj_with_mqa",
            "kv_b_proj",
            "o_proj",
            "gate_proj",
            "up_proj",
            "down_proj",
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

    # True if HF detected the deepspeed config -> ZeRO engine instead of DDP.
    print("DeepSpeed enabled:", trainer.is_deepspeed_enabled)
    print("Training started")
    model.print_trainable_parameters()
    trainer.train(resume_from_checkpoint=checkpoint_exists)

    # After train() the model is wrapped; should print "DeepSpeedEngine"
    # (not "DistributedDataParallel") if ZeRO is actually driving the training.
    print("Wrapped model:", type(trainer.model_wrapped).__name__)

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