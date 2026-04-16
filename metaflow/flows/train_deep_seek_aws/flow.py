from metaflow import FlowSpec, step, batch, torchrun, current, environment
import config
import store
from transformers import AutoTokenizer
from gpu_profile import gpu_profile

_DOCKER_IMAGE = "alejovasquero/cuda-metaflow:latest"

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


    @batch(
        gpu=1,
        cpu=1,
        memory=16000,
        image=_DOCKER_IMAGE
    )
    @step
    def start(self):
        self.next(self.load_dataset)

    @gpu_profile()
    @batch(
        gpu=1,
        cpu=8,
        memory=60000,
        image=_DOCKER_IMAGE
    )
    @step
    def load_dataset(self):
        print("Checking if dataset exists")
        if not self.data_store.already_exists():
            from unsloth import FastLanguageModel

            perfect_blend_dataset = self.data_store.load_from_hugging_face(dataset_path=self.data_config.hugging_face_name)
            print("Dataset downloaded from hugging face...")

            print("Loading model tokenizer...")
            tokenizer = AutoTokenizer.from_pretrained(self.training_config.model_name)

            print("Tokenizing dataset...")
            perfect_blend_dataset = self.data_store.format_and_tokenize(dataset=perfect_blend_dataset, tokenizer=tokenizer)

            # chunk and pack the dataset, whatever that means
            print("Uploading tokenized dataset to S3...")
            perfect_blend_dataset.save_to_disk(self.data_config.local_path)
            self.data_store.upload(local_path=self.data_config.local_path)
        else:
            print("Dataset already found in S3. Skipping re upload")

        self.next(self.train, num_parallel=2)

    @gpu_profile()
    @environment(vars={
        "AWS_METADATA_SERVICE_TIMEOUT": "1"
    })
    @batch(
        gpu=1,
        cpu=7,
        memory=60000,
        image=_DOCKER_IMAGE,
    )
    @torchrun
    @step
    def train(self):
        print("Downloading tokenized dataset...")

        self.data_store.download(local_path=self.data_config.local_path)
        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": self.data_config.local_path,
                "model_id": self.training_config.model_name,
                "bf16": True,
                "learning_rate": 2e-4,
                "output_dir": "/tmp/model/deepseekv2lite",
                "overwrite_output_dir": True,
                "warmup_steps": 5,
                "weight_decay": 0.01, 
                "packing": False,
                "report_to": "tensorboard",
                "logging_dir": "/tmp/model/deepseeklitev2history",
                "logging_steps": 1,
                "per_device_train_batch_size": 4,
                "num_train_epochs": 1,
                "gradient_accumulation_steps": 4,
                "max_steps": 3,
                "save_steps": 100,
                "seed": 42,
                "data_seed": 42,
            },
            nproc_per_node=1,
        )

        self.results_store.upload(local_path="/tmp/results")

        self.next(self.join)

    @step
    def join(self, inputs):
        self.next(self.end)

    @batch(
        gpu=1,
        cpu=1,
        memory=16000,
        image=_DOCKER_IMAGE
    )
    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlow()