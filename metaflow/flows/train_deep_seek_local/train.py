import unsloth

from unsloth import FastLanguageModel, is_bfloat16_supported
from trl import SFTTrainer
from transformers import TrainingArguments


def train_model(model_name: str) -> None:
    model, tokenizer = FastLanguageModel.from_pretrained(model_name=model_name)

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
    )

    max_seq_length = 2048 
    trainer = SFTTrainer(
        model = model,
        tokenizer = tokenizer,
        train_dataset = [],
        dataset_text_field = "text",
        max_seq_length = max_seq_length,
        dataset_num_proc = 2,
        args = TrainingArguments(
            per_device_train_batch_size = 2,
            gradient_accumulation_steps = 4,
            warmup_steps = 5,
            max_steps = 60,
            learning_rate = 2e-4,
            fp16 = not is_bfloat16_supported(),
            bf16 = is_bfloat16_supported(),
            logging_steps = 1,
            optim = "adamw_8bit",
            weight_decay = 0.01,
            lr_scheduler_type = "linear",
            seed = 3407,
            output_dir = "outputs",
        ),
    )



def main():
    train_model()
