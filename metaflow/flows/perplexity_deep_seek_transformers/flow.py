from metaflow import FlowSpec, step, batch
import os
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

class PerplexityFlow(FlowSpec):
    """Compute the perplexity of the BASE model on the code validation split.

    Expects the eval dataset already tokenized and stored in S3 under
    `code/validation` (produced by DeepSeekFlow.prepare_code_dataset). No
    training happens here; the base model is evaluated as-is.
    """

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
        return store.ResultsStore(self.data_config.perplexity_results_s3_prefix)

    @batch(
        gpu=1,
        cpu=1,
        memory=16000,
        image=_DOCKER_IMAGE
    )
    @step
    def start(self):
        self.next(self.prepare_eval_dataset)

    @batch(
        gpu=1,
        cpu=1,
        memory=16000,
        image=_DOCKER_IMAGE
    )
    @step
    def prepare_eval_dataset(self):
        """Build the deterministic code validation split (same rows as the
        transformers experiment) as a prompt/completion dataset and leave it in
        S3. The SFTTrainer applies the chat template and tokenizes at eval time;
        the prompt is masked so perplexity is computed over the assistant's
        completion only.
        """
        cfg = self.data_config

        if self.data_store.already_exists(store_key=cfg.val_text_store_key):
            print("Eval split already in S3. Skipping re build")
        else:
            print("Building deterministic code validation split from Perfect Blend...")
            _, val_dataset = self.data_store.load_code_split(
                dataset_path=cfg.hugging_face_name,
                code_source=cfg.code_source,
                n_train=cfg.code_n_train,
                n_val=cfg.code_n_val,
                seed=cfg.code_seed,
            )

            print("Loading model tokenizer...")
            tokenizer = AutoTokenizer.from_pretrained(self.training_config.model_name)

            print(f"Formatting validation split to prompt/completion ({len(val_dataset)} examples)...")
            val_pc = self.data_store.format_prompt_completion(
                dataset=val_dataset, tokenizer=tokenizer, max_length=cfg.eval_max_length
            )

            val_pc.save_to_disk(cfg.val_text_local_path)
            print(f"Uploading eval split to S3 ({cfg.val_text_store_key})...")
            self.data_store.upload(
                local_path=cfg.val_text_local_path, store_key=cfg.val_text_store_key
            )

        self.next(self.compute_perplexity)

    @gpu_profile()
    @batch(
        gpu=1,
        cpu=1,
        # DeepSeek-V2-Lite is ~15.7B params: the bf16 checkpoint is ~31GB across
        # 4 shards. Even loading in 4-bit, from_pretrained reads each shard's raw
        # weights into host RAM before quantizing/moving to GPU, so 16GB gets
        # SIGKILL'd (-9) by the OOM killer mid-shard. Give it real headroom.
        memory=60000,
        image=_DOCKER_IMAGE
    )
    @step
    def compute_perplexity(self):
        cfg = self.data_config

        print("Downloading code validation text split...")
        self.data_store.download(
            local_path=cfg.val_text_local_path,
            store_key=cfg.val_text_store_key,
        )

        from metaflow import TorchrunSingleNodeMultiGPU

        executor = TorchrunSingleNodeMultiGPU()
        # Resolve the entrypoint next to this flow file so it works both locally
        # (repo root) and on @batch, where Metaflow flattens the flow's files to
        # the package root (/workspace/metaflow/perplexity.py).
        entrypoint = os.path.join(
            os.path.dirname(os.path.abspath(__file__)), "perplexity.py"
        )
        executor.run(
            entrypoint=entrypoint,
            entrypoint_args={
                "model_id": self.training_config.model_name,
                "dataset_path": cfg.val_text_local_path,
                "output_dir": "/tmp/results",
                "bf16": True,
                "per_device_eval_batch_size": 1,
                # Keep equal to the training max_length for a fair comparison.
                "max_length": cfg.eval_max_length,
                # Prompt/completion dataset -> mask the prompt, score only the
                # assistant completion (assistant-only perplexity).
                "completion_only_loss": True,
                "report_to": "none",
            },
            nproc_per_node=1,
        )

        print("Uploading perplexity results to S3...")
        self.results_store.upload(local_path="/tmp/results")

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
    PerplexityFlow()
