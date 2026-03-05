import unsloth
import dataclasses
import datasets
from unsloth import FastLanguageModel
from trl import SFTTrainer, SFTConfig
from transformers import HfArgumentParser
from transformers import TrainerCallback
import json

@dataclasses.dataclass
class ScriptArguments:
    model_id: str
    dataset_path: str


class MetaflowBridge(TrainerCallback):
    def on_log(self, args, state, control, logs=None, **kwargs):
        print(logs)
        if logs and "loss" in logs:
            # Write only the necessary data for the graph
            with open("live_stats.json", "w") as f:
                json.dump({"step": state.global_step, "loss": logs["loss"]}, f)



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
    trainer.train(resume_from_checkpoint=True)
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