from metaflow import FlowSpec, step, current, Parameter
import data_store

import config

from unsloth import is_bfloat16_supported

class DeepSeepR1CPUTraining(FlowSpec):
    model_config = config.DeepSeekConfig()

    @step
    def start(self):
        self.next(self.load_dataset)


    @step
    def load_dataset(self):
        print("Preloading dataset in local path")
        self.ds_config = config.DataStoreConfig()
        data_store.DataStore.load_from_hugging_face(self.ds_config)
        self.next(self.train_model)

    @step
    def train_model(self):
        print("Running training script...")

        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": config.DataStoreConfig().local_path
            },
            master_port="41000",
        )
        self.next(self.end)


    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeepR1CPUTraining()