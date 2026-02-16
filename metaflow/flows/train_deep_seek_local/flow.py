from metaflow import FlowSpec, step, current, kubernetes, pypi, S3
import config
import store

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

    @step
    def start(self):
        self.next(self.load_dataset)

    @step
    def load_dataset(self):
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

        if not self.data_store.already_exists():
            print("Uploading tokenized dataset to S3...")
            perfect_blend_dataset.save_to_disk(self.data_config.local_path)
            self.data_store.upload(local_path=self.data_config.local_path)
        else:
            print("Dataset already found in S3. Skipping re upload.")

        self.next(self.train)

    
    @kubernetes(
        image="registry.hub.docker.com/pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime",
        cpu=1,
        memory=2000,
        shared_memory=8000,
    )
    @step
    def train(self):
        # download dataset from s3


        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": self.data_config.local_path,
                # pass the full model args. All args of the model and dataset should be here
            },
            nproc_per_node=1,
            master_port=41000,
        )
        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlow()