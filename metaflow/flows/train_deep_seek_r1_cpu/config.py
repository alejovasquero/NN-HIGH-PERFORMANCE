import dataclasses

@dataclasses.dataclass(frozen=True)
class DeepSeekConfig:
    model_name: str = "deepseek-ai/DeepSeek-V2-Lite"
