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

    # Deterministic code-question split carved out of Perfect Blend.
    code_source: str = "theblackcat102/evol-codealpaca-v1"
    code_seed: int = 42
    code_n_train: int = 4000
    code_n_val: int = 400  # only used to keep the deterministic split identical
    # Code training split in prompt/completion format (completion-only training),
    # sharing the same S3 object as the AWS flow. No validation split here.
    train_pc_store_key: str = "code/train_pc"
    train_pc_local_path: str = "/tmp/code-blend/train_pc"
    train_max_length: int = 2048