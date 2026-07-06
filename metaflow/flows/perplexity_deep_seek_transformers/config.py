import dataclasses


@dataclasses.dataclass(frozen=True)
class TrainingConfig:
    model_name: str = "deepseek-ai/DeepSeek-V2-Lite"
    learning_rate: float = 3e-4
    fp16: bool = True
    master_port: int = 1234


@dataclasses.dataclass(frozen=True)
class DataStoreConfig:
    hugging_face_name: str = "mlabonne/open-perfectblend"
    local_path = "/tmp/open-perfectblend"
    s3_prefix = "perfect-blend"
    results_s3_prefix = "results"
    perplexity_results_s3_prefix = "perplexity-results"

    # Deterministic code-question split carved out of Perfect Blend.
    code_source: str = "theblackcat102/evol-codealpaca-v1"
    code_seed: int = 42
    code_n_train: int = 4000
    code_n_val: int = 400 
    # S3 store keys (under s3_prefix) and matching local paths for each split.
    train_store_key: str = "code/train"
    val_store_key: str = "code/validation"
    train_local_path: str = "/tmp/code-blend/train"
    val_local_path: str = "/tmp/code-blend/validation"

    # Text (chat-template-formatted, NOT tokenized) version of the eval split.
    # The SFTTrainer tokenizes it at eval time with the same max_length/collator
    # as training, so perplexity = exp(eval_loss) is comparable to the
    # fine-tuned model's eval_loss. Stored under a distinct key so it does not
    # clobber the tokenized `code/validation` produced by the training flow.
    val_text_store_key: str = "code/validation_text"
    val_text_local_path: str = "/tmp/code-blend/validation_text"
    # Must equal the training max_length for a like-for-like comparison.
    eval_max_length: int = 2048