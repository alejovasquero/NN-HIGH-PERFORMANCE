from metaflow import FlowSpec, step, current, torchrun, kubernetes, pypi, S3
import config
import data_store

class DeepSeekFlow(FlowSpec):
    training_config = config.TrainingConfig()
    data_config = config.DataStoreConfig()

    @step
    def start(self):
        self.next(self.load_dataset)

    @step
    def load_dataset(self):
        data_st = data_store.DataStore()
        perfect_blend_dataset = data_st.load_from_hugging_face(dataset_path=self.data_config.hugging_face_name)
        perfect_blend_dataset.save_to_disk(self.data_config.local_path)


        with S3(run=self) as s3:
            if s3.list_paths(keys=[self.data_config.local_path]) == 0:
                print("Uploading dataset to S3")
                s3.put_files(self.data_config.local_path)

        self.next(self.train)

    
    @kubernetes(
        image="registry.hub.docker.com/pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime",
        cpu=1,
        memory=2000,
        shared_memory=8000,
    )
    @step
    def train(self):
        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": self.data_config.local_path
            },
            nproc_per_node=1,
        )
        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlow()