from metaflow import FlowSpec, step, batch, torchrun, current, environment
import json
import config
import store
from transformers import AutoTokenizer
from gpu_profile import gpu_profile

_DOCKER_IMAGE = "alejovasquero/cuda-metaflow:latest"

# DeepSpeed ZeRO-2 (no offload, no fused optimizer -> no CUDA op compilation, runs
# on a runtime image without nvcc). Fixed reduce-scatter over all trainable params
# every step (no DDP find_unused coordination) -> handles MoE routing that breaks
# plain DDP. Values MUST match the train entrypoint_args below:
#   per_device_train_batch_size=1, gradient_accumulation_steps=4, 2 nodes
#   -> train_batch_size = 1 * 4 * 2 = 8
_DS_CONFIG = {
    "bf16": {"enabled": True},
    "zero_optimization": {
        "stage": 2,
        "allgather_partitions": True,
        "allgather_bucket_size": 2e8,
        "overlap_comm": True,
        "reduce_scatter": True,
        "reduce_bucket_size": 2e8,
        "contiguous_gradients": True,
    },
    "train_micro_batch_size_per_gpu": 1,
    "gradient_accumulation_steps": 4,
    "train_batch_size": 8,
    "gradient_clipping": 1.0,
}
_DS_CONFIG_PATH = "/tmp/ds_config.json"

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
        self.next(self.prepare_code_dataset)

    @gpu_profile()
    @batch(
        gpu=1,
        cpu=8,
        memory=60000,
        image=_DOCKER_IMAGE
    )
    @step
    def prepare_code_dataset(self):
        """Build the deterministic code split as prompt/completion and store it.

        Both train (4000) and eval (400) are uploaded to S3 in prompt/completion
        format so training is completion-only and the final perplexity is
        assistant-only (comparable to the base perplexity flow).
        """
        cfg = self.data_config
        train_exists = self.data_store.already_exists(store_key=cfg.train_pc_store_key)
        val_exists = self.data_store.already_exists(store_key=cfg.val_pc_store_key)

        if train_exists and val_exists:
            print("Code train/val prompt-completion splits already in S3. Skipping")
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

            for name, split_ds, local_path, store_key in (
                ("train", train_dataset, cfg.train_pc_local_path, cfg.train_pc_store_key),
                ("validation", val_dataset, cfg.val_pc_local_path, cfg.val_pc_store_key),
            ):
                print(f"Formatting {name} split to prompt/completion ({len(split_ds)} examples)...")
                pc = self.data_store.format_prompt_completion(dataset=split_ds, tokenizer=tokenizer, max_length=cfg.eval_max_length)
                pc.save_to_disk(local_path)
                print(f"Uploading {name} split to S3 ({store_key})...")
                self.data_store.upload(local_path=local_path, store_key=store_key)

        self.next(self.train, num_parallel=2)

    @gpu_profile()
    # NCCL_DEBUG=INFO + SUBSYS=COLL logueaba CADA colectivo. _broadcast_model
    # encola miles de broadcasts (MoE) -> miles de write() a stdout, que es un
    # pipe hacia mflog; el pipe se llena y write() se BLOQUEA dentro del enqueue
    # de NCCL -> el broadcast se cuelga (confirmado con py-spy --native: MainThread
    # parado en ncclDebugLog -> fwrite -> write). WARN elimina ese flood.
    @environment(vars={
        "NCCL_DEBUG": "WARN",
        "NCCL_SOCKET_IFNAME": "eth0",
        "DEEPSPEED_TIMEOUT": "420",
        "NCCL_BUFFSIZE": "67108864",
        "NCCL_TIMEOUT": "420",
        "NCCL_SOCKET_NTHREADS": "4",
        "NCCL_NSOCKS_PERTHREAD": "4",
        "ACCELERATE_DDP_TIMEOUT": "420",
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
        print("Downloading prompt-completion train/val splits...")
        node_index = current.parallel.node_index
        cfg = self.data_config

        self.data_store.download(local_path=cfg.train_pc_local_path, store_key=cfg.train_pc_store_key)
        self.data_store.download(local_path=cfg.val_pc_local_path, store_key=cfg.val_pc_store_key)

        # Write the DeepSpeed config on this node so train.py can point at it.
        with open(_DS_CONFIG_PATH, "w") as f:
            json.dump(_DS_CONFIG, f)

        current.torch.run(
            entrypoint="train.py",
            entrypoint_args={
                "dataset_path": cfg.train_pc_local_path,
                "eval_dataset_path": cfg.val_pc_local_path,
                "model_id": self.training_config.model_name,
                "bf16": True,
                "learning_rate": 2e-4,
                "output_dir": f"/tmp/model/deepseekv2lite_{node_index}",
                "overwrite_output_dir": True,
                "warmup_steps": 5,
                "weight_decay": 0.01,
                "packing": False,
                # Completion-only: mask the prompt, train + score only the
                # assistant completion. Perplexity is computed once at the end.
                "completion_only_loss": True,
                "max_length": cfg.eval_max_length,
                "logging_dir": f"/tmp/model/deepseeklitev2history_{node_index}",
                "logging_steps": 1,
                "report_to": "none",
                # DeepSpeed ZeRO-2 config (written above); handles the MoE routing
                # that breaks plain DDP.
                "deepspeed": _DS_CONFIG_PATH,
                "per_device_train_batch_size": 1,
                # Eval con batch 1: el default de HF (8) hace un tensor de logits
                # [8, 2048, 102400] (~3.8 GB) que OOMea la L40S al calcular la
                # perplexity final. Con 1 baja a ~0.5 GB y cabe.
                "per_device_eval_batch_size": 1,
                "num_train_epochs": 1,
                "gradient_accumulation_steps": 4,
                "max_steps": 10, # TODO put 500
                "save_steps": 100,
                "seed": 42,
                "data_seed": 42,
                "dataloader_drop_last": False,
                # Fast fail: if a collective still hangs, dump at ~7 min (not 30).
            },
            nproc_per_node=1,
        )

        self.results_store.upload(local_path="/tmp/results")

        self.next(self.join)

    @batch(
        gpu=1,
        cpu=1,
        memory=16000,
        image=_DOCKER_IMAGE
    )
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