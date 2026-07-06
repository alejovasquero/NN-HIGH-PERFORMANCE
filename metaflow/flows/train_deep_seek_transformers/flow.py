from metaflow import FlowSpec, step, batch, torchrun, current, environment, pypi
import config
import store
from transformers import AutoTokenizer
from gpu_profile import gpu_profile

_DOCKER_IMAGE = "alejovasquero/cuda-metaflow:latest"
_PACKAGES = {
    "datasets": "4.3.0",
    "transformers": "4.57.1",
    "unsloth": "2025.11.2",
    "tensorboard": "2.20.0",
    "matplotlib": "3.10.8",
}

class DeepSeekFlow(FlowSpec):

    @property
    def data_config(self) -> config.DataStoreConfig:
        return config.DataStoreConfig()
    
    @property
    def training_config(self) -> config.TrainingConfig:
        return config.TrainingConfig()

    @property
    def data_store(self) -> store.DataStore:
        return store.DataStore(self.data_config.s3_prefix)
    
    @property
    def results_store(self) -> store.ResultsStore:
        return store.ResultsStore(self.data_config.results_s3_prefix)


    @pypi(packages=_PACKAGES)
    @step
    def start(self):
        self.next(self.prepare_code_dataset)

    @gpu_profile()
    @pypi(packages=_PACKAGES)
    @step
    def prepare_code_dataset(self):
        """Build the deterministic code TRAIN split (prompt/completion) in S3.

        Filters Perfect Blend to the code source, takes the same deterministic
        4000-example train split as the other flows, formats it to
        prompt/completion (for completion-only training), and uploads it to
        `code/train_pc`. No validation split is produced here.
        """
        cfg = self.data_config

        if self.data_store.already_exists(store_key=cfg.train_pc_store_key):
            print("Code train prompt-completion split already in S3. Skipping re upload")
        else:
            print("Building deterministic code split from Perfect Blend...")
            train_dataset, _ = self.data_store.load_code_split(
                dataset_path=cfg.hugging_face_name,
                code_source=cfg.code_source,
                n_train=cfg.code_n_train,
                n_val=cfg.code_n_val,
                seed=cfg.code_seed,
            )

            print("Loading model tokenizer...")
            tokenizer = AutoTokenizer.from_pretrained(self.training_config.model_name)

            print(f"Formatting train split to prompt/completion ({len(train_dataset)} examples)...")
            train_pc = self.data_store.format_prompt_completion(
                dataset=train_dataset, tokenizer=tokenizer, max_length=cfg.train_max_length
            )
            train_pc.save_to_disk(cfg.train_pc_local_path)
            print(f"Uploading train split to S3 ({cfg.train_pc_store_key})...")
            self.data_store.upload(
                local_path=cfg.train_pc_local_path, store_key=cfg.train_pc_store_key
            )

        self.next(self.train)

    @gpu_profile()
    @pypi(packages=_PACKAGES)
    @step
    def train(self):
        print("Downloading prompt-completion code train split...")

        self.data_store.download(
            local_path=self.data_config.train_pc_local_path,
            store_key=self.data_config.train_pc_store_key,
        )

        from metaflow import TorchrunSingleNodeMultiGPU

        executor = TorchrunSingleNodeMultiGPU()

        executor.run(
            entrypoint="flows/train_deep_seek_transformers/train.py",
            entrypoint_args={
                "dataset_path": self.data_config.train_pc_local_path,
                "model_id": self.training_config.model_name,
                "bf16": True,
                "learning_rate": 2e-4,
                "output_dir": f"/tmp/model/deepseekv2lite_{1}",
                "overwrite_output_dir": True,
                "warmup_steps": 5,
                "weight_decay": 0.01,
                "packing": False,
                # Completion-only: mask the prompt, train only on the assistant
                # completion. No validation here.
                "completion_only_loss": True,
                "max_length": self.data_config.train_max_length,
                "logging_dir": f"/tmp/model/deepseeklitev2history_{1}",
                "logging_steps": 1,
                "report_to": "none",
                "per_device_train_batch_size": 1,
                "num_train_epochs": 1,
                "gradient_accumulation_steps": 4,
                "max_steps": 1000,
                "save_steps": 100,
                "seed": 42,
                "data_seed": 42,
            },
            nproc_per_node=1,
        )

        self.results_store.upload(local_path="/tmp/results")

        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlow()