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