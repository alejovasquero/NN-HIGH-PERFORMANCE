import unsloth
import dataclasses
import datasets
from unsloth import FastLanguageModel
from trl import SFTTrainer, SFTConfig
from transformers import HfArgumentParser
from transformers import TrainerCallback
import json
import os
import csv

@dataclasses.dataclass
class ScriptArguments:
    model_id: str
    dataset_path: str


class MetaflowBridge(TrainerCallback):

    def __init__(self, filename="training_log.csv"):
        self.filename = filename
        if not os.path.exists(self.filename):
            with open(self.filename, 'w', newline='') as f:
                writer = csv.writer(f)
                writer.writerow(["step", "loss", "learning_rate"])

    def on_log(self, args, state, control, logs=None, **kwargs):
        print("Captured log", logs)
        if logs and "loss" in logs:
            with open(self.filename, 'a', newline='') as f:
                writer = csv.writer(f)
                writer.writerow([
                    state.global_step, 
                    logs["loss"], 
                    logs.get("learning_rate", 0)
                ])



def train_model(script_args: ScriptArguments, trainig_args: SFTConfig) -> None:
    dataset = datasets.load_from_disk(script_args.dataset_path)
    print(f"Dataset found with {dataset.features} features")
    print(trainig_args)

    model, tokenizer = FastLanguageModel.from_pretrained(model_name=script_args.model_id, load_in_4bit=True)
    model = FastLanguageModel.get_peft_model(
        model,
        r=16, 
        target_modules=[
            "q_proj",
            "k_proj",
            "v_proj",
            "o_proj",
            "gate_proj",
            "up_proj",
            "down_proj",
        ],
        lora_alpha=16,
        lora_dropout=0, 
        bias="none", 
        use_gradient_checkpointing="unsloth", 
        random_state=3407,
        use_rslora=False, 
        loftq_config=None,
        max_seq_length=2048,
    )

    trainer = SFTTrainer(
        model = model,
        processing_class = tokenizer,
        train_dataset = dataset,
        args = trainig_args,
        callbacks=[MetaflowBridge()]
    )


    checkpoint_exists = False
    if os.path.exists(trainig_args.output_dir):
        checkpoints = [d for d in os.listdir(trainig_args.output_dir) if "checkpoint" in d]
        checkpoint_exists = len(checkpoints) > 0

    trainer.train(resume_from_checkpoint=checkpoint_exists)
    trainer.save_model()

    model.save_pretrained_gguf(
        "model_gguf", 
        tokenizer, 
        quantization_method = "q4_k_m"
    )



def main():
    parser = HfArgumentParser([ScriptArguments, SFTConfig])
    script_args, training_args = parser.parse_args_into_dataclasses()
    train_model(script_args, training_args)


if __name__ == "__main__":
    main()