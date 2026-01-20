from metaflow import FlowSpec, step, current, torchrun, kubernetes
import config

class DeepSeekFlow(FlowSpec):
    training_config = config.TrainingConfig()

    @step
    def start(self):
        self.next(self.train_multinode, num_parallel=2)

    @kubernetes(
        image="registry.hub.docker.com/pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime",
        cpu=1,
        memory=2000,
        shared_memory=8000,
    )
    @torchrun
    @step
    def train_multinode(self):
        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": config.DataStoreConfig().local_path
            },
            nproc_per_node=1,
        )
        self.next(self.join)

    @step
    def join(self, inputs):
        print("Finished training. Wrapping up...")
        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeekFlow()