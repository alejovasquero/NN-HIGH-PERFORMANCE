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

    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.bfloat16,
        bnb_4bit_use_double_quant=True,
    )
    local_rank = int(os.environ.get("LOCAL_RANK", 0))
    device_map = {"": local_rank}

    model = AutoModelForCausalLM.from_pretrained(
        script_args.model_id,
        quantization_config=bnb_config,
        device_map=device_map,
        trust_remote_code=True,
    )
    tokenizer = AutoTokenizer.from_pretrained(script_args.model_id)
    model.eval()
    print("Base model loaded (no LoRA, no training)")

    # transformers' Trainer refuses a purely-quantized (4-bit) model unless it
    # has trainable adapters. Attach a LoRA adapter to satisfy the check. LoRA is
    # zero-initialized (B = 0), so W + BA = W: the adapter changes nothing and
    # this measures the BASE model's perplexity exactly.
    peft_config = LoraConfig(
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
        lora_dropout=0,
        bias="none",
        task_type="CAUSAL_LM",
    )

    # We only call evaluate(), but SFTTrainer.__init__ unconditionally reads a
    # sample from train_dataset (next(iter(train_dataset))), so it must not be
    # None. Pass the same dataset as train_dataset — it is never trained on.
    trainer = SFTTrainer(
        model=model,
        processing_class=tokenizer,
        train_dataset=dataset,
        eval_dataset=dataset,
        args=eval_args,
        peft_config=peft_config,
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
