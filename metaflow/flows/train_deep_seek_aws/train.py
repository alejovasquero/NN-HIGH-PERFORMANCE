import dataclasses
import datasets
from trl import SFTTrainer, SFTConfig
from transformers import HfArgumentParser
from transformers import TrainerCallback
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


class MetaflowBridge(TrainerCallback):

    def __init__(self, batch_size: int, filename="/tmp/results/training_log.csv"):
        self.filename = filename
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



def train_model(script_args: ScriptArguments, training_args: SFTConfig) -> None:
    dataset = datasets.load_from_disk(script_args.dataset_path)
    print(f"Dataset found with {dataset.features} features")
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

    model = AutoModelForCausalLM.from_pretrained(
        script_args.model_id,
        quantization_config=bnb_config,
        device_map=device_map,
        trust_remote_code=True
    )
    tokenizer = AutoTokenizer.from_pretrained(script_args.model_id)
    tokenizer.pad_token = tokenizer.eos_token
    model = prepare_model_for_kbit_training(model)

    lora_config = LoraConfig(
        r=16,
        lora_alpha=16,
        target_modules=["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
        lora_dropout=0,
        bias="none",
        task_type="CAUSAL_LM"
    )
    model = get_peft_model(model, lora_config)

    print("Model loaded")

    trainer = SFTTrainer(
        model=model,
        tokenizer=tokenizer,
        train_dataset=dataset,
        args=training_args,
        callbacks=[MetaflowBridge(batch_size=training_args.per_device_train_batch_size)],
    )
    print("Trainer created")


    checkpoint_exists = False
    if os.path.exists(training_args.output_dir):
        checkpoints = [d for d in os.listdir(training_args.output_dir) if "checkpoint" in d]
        checkpoint_exists = len(checkpoints) > 0

    print("Training started")
    trainer.train(resume_from_checkpoint=checkpoint_exists)


def main():
    parser = HfArgumentParser([ScriptArguments, SFTConfig])
    script_args, training_args = parser.parse_args_into_dataclasses()
    train_model(script_args, training_args)


if __name__ == "__main__":
    main()