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
        """Build a deterministic code-question train/val split and leave it in S3.

        Filters Perfect Blend to the code source, carves a disjoint
        4000-train / 400-val split (fixed seed), tokenizes each split, and
        uploads them to S3 under `code/train` and `code/validation`.
        """
        cfg = self.data_config
        train_exists = self.data_store.already_exists(store_key=cfg.train_store_key)
        val_exists = self.data_store.already_exists(store_key=cfg.val_store_key)

        if train_exists and val_exists:
            print("Code train/val splits already in S3. Skipping re upload")
        else:
            print("Building deterministic code split from Perfect Blend...")
            train_dataset, val_dataset = self.data_store.load_code_split(
                dataset_path=cfg.hugging_face_name,
                code_source=cfg.code_source,
                n_train=cfg.code_n_train,
                n_val=cfg.code_n_val,
                seed=cfg.code_seed,
            )

            print("Loading model tokenizer...")
            tokenizer = AutoTokenizer.from_pretrained(self.training_config.model_name)

            for split_name, split_ds, local_path, store_key in (
                ("train", train_dataset, cfg.train_local_path, cfg.train_store_key),
                ("validation", val_dataset, cfg.val_local_path, cfg.val_store_key),
            ):
                print(f"Tokenizing {split_name} split ({len(split_ds)} examples)...")
                tokenized = self.data_store.format_and_tokenize(
                    dataset=split_ds, tokenizer=tokenizer
                )
                print(f"Uploading {split_name} split to S3 ({store_key})...")
                tokenized.save_to_disk(local_path)
                self.data_store.upload(local_path=local_path, store_key=store_key)

        self.next(self.train)

    @gpu_profile()
    @pypi(packages=_PACKAGES)
    @step
    def train(self):
        print("Downloading tokenized code train split...")

        self.data_store.download(
            local_path=self.data_config.train_local_path,
            store_key=self.data_config.train_store_key,
        )

        from metaflow import TorchrunSingleNodeMultiGPU

        executor = TorchrunSingleNodeMultiGPU()

        executor.run(
            entrypoint="flows/train_deep_seek_transformers/train.py",
            entrypoint_args={
                "dataset_path": self.data_config.train_local_path,
                "model_id": self.training_config.model_name,
                "bf16": True,
                "learning_rate": 2e-4,
                "output_dir": f"/tmp/model/deepseekv2lite_{1}",
                "overwrite_output_dir": True,
                "warmup_steps": 5,
                "weight_decay": 0.01, 
                "packing": False,
                "logging_dir": f"/tmp/model/deepseeklitev2history_{1}",
                "logging_steps": 1,
                "report_to": "none",
                "per_device_train_batch_size": 4,
                "num_train_epochs": 1,
                "gradient_accumulation_steps": 4,
                "max_steps": 125,
                "save_steps": 100,
                "seed": 42,
                "data_seed": 42,
                "max_seq_length": 512,
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