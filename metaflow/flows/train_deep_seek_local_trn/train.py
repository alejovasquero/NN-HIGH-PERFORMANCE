from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    default_data_collator,
    set_seed,
    TrainerCallback,
)
import datasets
import json

import dataclasses
 
from optimum.neuron import NeuronHfArgumentParser as HfArgumentParser
from optimum.neuron import NeuronTrainer as Trainer
from optimum.neuron import NeuronTrainingArguments as TrainingArguments
from optimum.neuron.distributed import lazy_load_for_parallelism

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



def train_model(script_args: ScriptArguments, training_args: TrainingArguments) -> None:
    dataset = datasets.load_from_disk(script_args.dataset_path)
    print(f"Dataset found with {dataset.shape}")
    print(training_args)

    tokenizer = AutoTokenizer.from_pretrained(script_args.model_id)

    with lazy_load_for_parallelism(
        tensor_parallel_size=training_args.tensor_parallel_size
    ):
        model = AutoModelForCausalLM.from_pretrained(
            script_args.model_id,
            torch_dtype="auto",
            low_cpu_mem_usage=True,
            use_cache=False if training_args.gradient_checkpointing else True,
        )

    print("Model loaded from pretrained")

    trainer = Trainer(
        model=model,
        tokenizer=tokenizer,
        args=training_args,
        train_dataset=dataset,
    )
    trainer.train(resume_from_checkpoint=True)
    trainer.save_model()

    model.save_pretrained_gguf(
        "model_gguf", 
        tokenizer, 
        quantization_method = "q4_k_m"
    )



def main():
    parser = HfArgumentParser([ScriptArguments, TrainingArguments])
    script_args, training_args = parser.parse_args_into_dataclasses()
    train_model(script_args, training_args)


if __name__ == "__main__":
    main()