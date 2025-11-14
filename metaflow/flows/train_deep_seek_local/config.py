import dataclasses

@dataclasses.dataclass(frozen=True)
class DeepSeekConfig:
    model_name: str = "deepseek-ai/DeepSeek-V2-Lite"
    dataset_name: str = "mlabonne/open-perfectblend"



class DataStoreConfig:
    hugging_face_name: str = "mlabonne/open-perfectblend"
    local_path = "/tmp/open-perfectblend"