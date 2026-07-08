"""Perplexity of the BASE model on the code validation split (route A).

Uses the SFTTrainer's own evaluation loop so the number is computed exactly like
the fine-tuned model's `eval_loss` would be (same chat-template text, same
tokenization at `max_length`, same collator/masking):

    perplexity = exp(eval_loss)

The base model is loaded in 4-bit with NO LoRA and NO training — it is evaluated
as-is. The eval dataset is the chat-template-formatted TEXT split produced by
`prepare_eval_dataset`; the trainer tokenizes it at eval time.
"""

import csv
import dataclasses
import json
import math
import os

import datasets
import torch
from peft import LoraConfig
from trl import SFTTrainer, SFTConfig
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    BitsAndBytesConfig,
    HfArgumentParser,
)


@dataclasses.dataclass
class ScriptArguments:
    model_id: str
    dataset_path: str


def main() -> None:
    parser = HfArgumentParser([ScriptArguments, SFTConfig])
    script_args, eval_args = parser.parse_args_into_dataclasses()

    dataset = datasets.load_from_disk(script_args.dataset_path)
    print(f"Eval dataset: {len(dataset)} examples, features {dataset.features}")

    # Drop the `text` column: SFTTrainer's collator routes any dataset that has
    # `text` to language-modeling mode and raises with completion_only_loss=True.
    # We keep only prompt/completion so the loss is masked to the assistant turn.
    dataset = dataset.select_columns(["prompt", "completion"])
    print(eval_args)

    local_rank = int(os.environ.get("LOCAL_RANK", 0))

    # Load in 4-bit (NF4 + double quant), same config as the fine-tuned QLoRA
    # base -> base vs fine-tuned perplexity comparable, and both match the local
    # 4-bit setup. (The earlier "4-bit hangs DeepSpeed" was the NCCL_DEBUG pipe
    # deadlock, now fixed; and this base eval is single-node, no DeepSpeed.)
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
    model.eval()
    print("Base model loaded in 4-bit (no LoRA, no training)")

    # No peft_config: the model is not quantized, so the Trainer's "cannot train
    # a purely quantized model" guard doesn't apply -> the zero-LoRA trick isn't
    # needed. SFTTrainer.__init__ still reads a sample from train_dataset, so
    # pass the eval dataset there too (it is never trained on).
    trainer = SFTTrainer(
        model=model,
        processing_class=tokenizer,
        train_dataset=dataset,
        eval_dataset=dataset,
        args=eval_args,
    )

    metrics = trainer.evaluate()
    eval_loss = metrics["eval_loss"]
    perplexity = math.exp(eval_loss)
    print(
        f"\nBASE MODEL  eval_loss={eval_loss:.6f}  perplexity={perplexity:.6f}  "
        f"(max_length={eval_args.max_length}, {len(dataset)} sequences)"
    )

    os.makedirs(eval_args.output_dir, exist_ok=True)
    result = {
        "model_id": script_args.model_id,
        "dataset_path": script_args.dataset_path,
        "eval_loss": eval_loss,
        "perplexity": perplexity,
        "num_sequences": len(dataset),
        "max_length": eval_args.max_length,
    }
    with open(os.path.join(eval_args.output_dir, "perplexity.json"), "w") as f:
        json.dump(result, f, indent=2)
    with open(os.path.join(eval_args.output_dir, "perplexity.csv"), "w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(list(result.keys()))
        writer.writerow(list(result.values()))


if __name__ == "__main__":
    main()
