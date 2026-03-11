from metaflow import FlowSpec, current, step, batch, torchrun, pypi
import config
import store


_PACKAGES = {
    "datasets": "4.3.0",
    "transformers": "4.57.1",
    "tensorboard": "2.20.0",
    "git+https://github.com/huggingface/optimum-neuron.git": "",
}

class DeepSeekFlowTrn(FlowSpec):

    @property
    def data_config(self) -> config.DataStoreConfig:
        return config.DataStoreConfig()
    
    @property
    def training_config(self) -> config.TrainingConfig:
        return config.TrainingConfig()

    @property
    def data_store(self) -> store.DataStore:
        return store.DataStore(self.data_config.s3_prefix)

    @pypi(packages=_PACKAGES)
    @step
    def start(self):
        self.next(self.load_dataset)

    @pypi(packages=_PACKAGES)
    @step
    def load_dataset(self):
        print("Checking if dataset exists")
        if not self.data_store.already_exists():
            from unsloth import FastLanguageModel

            perfect_blend_dataset = self.data_store.load_from_hugging_face(dataset_path=self.data_config.hugging_face_name)
            print("Dataset downloaded from hugging face...")

            print("Loading model tokenizer...")
            _, tokenizer = FastLanguageModel.from_pretrained(
                model_name=self.training_config.model_name,
                load_in_4bit=True,
            )

            print("Tokenizing dataset...")
            perfect_blend_dataset = self.data_store.format_and_tokenize(dataset=perfect_blend_dataset, tokenizer=tokenizer)

            # chunk and pack the dataset, whatever that means
            print("Uploading tokenized dataset to S3...")
            perfect_blend_dataset.save_to_disk(self.data_config.local_path)
            self.data_store.upload(local_path=self.data_config.local_path)
        else:
            print("Dataset already found in S3. Skipping re upload")

        self.next(self.train, num_parallel=2)

    @pypi(packages=_PACKAGES)
    @batch(
        trainium=16,
        cpu=96,
        memory=500000,
    )
    @torchrun
    @step
    def train(self):
        print("Downloading tokenized dataset...")

        self.data_store.download(local_path=self.data_config.local_path)
        current.torch.run(
            torchrun_args={"master_port": "41000"},
            entrypoint="run_clm.py",
            entrypoint_args={
                "dataset_path": self.data_config.local_path,
                "model_id": self.training_config.model_name,
                "bf16": True,
                "learning_rate": 2e-4,
                "output_dir": "/tmp/model/deepseekv2lite",
                "overwrite_output_dir": True,
                "packing": True,
                "report_to": "tensorboard",
                "logging_dir": "/tmp/model/deepseeklitev2history",
                "logging_steps": 2,
                "per_device_train_batch_size": 1,
                "num_train_epochs": 1,
                "gradient_accumulation_steps": 4,
                "max_steps": 500,
                "save_steps": 50,
                "gradient_checkpointing": True,
                "tensor_parallel_size": 8,
            },
        )
        self.next(self.join)

    @step
    def join(self, inputs) -> None:
        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlowTrn()