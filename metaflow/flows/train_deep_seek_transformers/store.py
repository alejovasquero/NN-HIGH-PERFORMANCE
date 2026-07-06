from typing import Any, Tuple
import os
import random
import shutil
from metaflow import S3
from random import randint

import datasets
from metaflow.metaflow_config import DATATOOLS_S3ROOT

from unsloth import standardize_sharegpt

class BaseStore:

    def __init__(self, s3_prefix: str):
        self._store_root = os.path.join(DATATOOLS_S3ROOT, s3_prefix)

    @staticmethod
    def _walk_directory(root):
        path_keys = []
        for path, _, files in os.walk(root):
            for name in files:
                path_keys.append(
                    (
                        os.path.relpath(os.path.join(path, name), root),
                        os.path.join(path, name),
                    )
                )
        return path_keys
    
    def _upload_directory(self, local_path: str, store_key: str = ""):
        final_path = os.path.join(self._store_root, store_key)
        with S3(s3root=final_path) as s3:
            s3.put_files(self._walk_directory(local_path))

    def upload(self, local_path: str, store_key: str = "") -> None:
        if os.path.isdir(local_path):
            self._upload_directory(local_path=local_path, store_key=store_key)
        else:
            final_path = os.path.join(self._store_root, store_key)
            with S3(s3root=final_path) as s3:
                s3.put_files(key_paths=[(local_path, local_path)])


    def download(self, local_path: str, store_key: str = "") -> None:

        os.makedirs(name=local_path, exist_ok=True)
        final_path = os.path.join(self._store_root, store_key)

        with S3(s3root=final_path) as s3:
            for s3obj in s3.get_all():
                print(s3obj)
                local_object_path = os.path.join(local_path, s3obj.key)
                shutil.move(s3obj.path, local_object_path)


    def already_exists(self, store_key: str = "") -> bool:
        final_path = os.path.join(self._store_root, store_key)
        with S3(s3root=final_path) as s3:
            print(final_path, s3.list_paths())
            if len(s3.list_paths()) == 0:
                return False
            return True

class DataStore(BaseStore):

    def load_from_hugging_face(self, dataset_path: str) -> datasets.Dataset:
        dataset = datasets.load_dataset(dataset_path, split="train[:4000]")
        return dataset

    def load_code_split(
        self,
        dataset_path: str,
        code_source: str,
        n_train: int,
        n_val: int,
        seed: int,
    ) -> Tuple[datasets.Dataset, datasets.Dataset]:
        """Deterministically carve a code-question train/validation split.

        Keeps only examples whose `source` is `code_source`, shuffles their
        indices with a fixed seed, and slices two DISJOINT splits. Running this
        with the same arguments always yields the exact same rows.
        """
        dataset = datasets.load_dataset(dataset_path, split="train")
        code_indices = [
            i for i, s in enumerate(dataset["source"]) if s == code_source
        ]
        print(f"Found {len(code_indices)} code examples for source {code_source}")

        needed = n_train + n_val
        if len(code_indices) < needed:
            raise ValueError(
                f"Not enough code examples: need {needed}, have {len(code_indices)}"
            )

        rng = random.Random(seed)
        rng.shuffle(code_indices)
        train_indices = sorted(code_indices[:n_train])
        val_indices = sorted(code_indices[n_train : n_train + n_val])
        assert set(train_indices).isdisjoint(val_indices), "train/val overlap!"

        train_dataset = dataset.select(train_indices)
        val_dataset = dataset.select(val_indices)
        print(f"Code split -> train: {len(train_dataset)}, val: {len(val_dataset)}")
        return train_dataset, val_dataset

    def format_prompt_completion(self, dataset: datasets.Dataset, tokenizer: Any, max_length: int) -> datasets.Dataset:
        """Split each conversation into `prompt` + `completion` (conversational).

        SFTTrainer uses this format for completion-only loss: the prompt (user
        turns) is masked, so training is scored ONLY over the assistant's
        completion. `text` is kept for inspection and MUST be dropped before the
        SFTTrainer (train.py does it).

        Examples whose prompt alone is >= max_length are dropped: after the
        collator truncates prompt+completion to max_length, no completion token
        would survive, leaving an all-masked sequence -> NaN loss.
        """
        dataset = standardize_sharegpt(dataset)

        def _split(sample: Any) -> dict:
            messages = sample["conversations"]
            text = tokenizer.apply_chat_template(
                messages, tokenize=False, add_generation_prompt=False
            )
            return {
                "text": text,
                "prompt": messages[:-1],
                "completion": [messages[-1]],
            }

        pc_dataset = dataset.map(
            _split,
            batched=False,
            keep_in_memory=True,
            remove_columns=list(dataset.features),
        )

        before = len(pc_dataset)
        pc_dataset = pc_dataset.filter(
            lambda ex: len(tokenizer.apply_chat_template(ex["prompt"])) < max_length
        )
        print(f"Dropped {before - len(pc_dataset)} examples with prompt >= {max_length} -> {len(pc_dataset)} remain")

        print("Sample prompt:", pc_dataset[0]["prompt"])
        print("Sample completion:", pc_dataset[0]["completion"])
        return pc_dataset

    def format_and_tokenize(self, dataset: datasets.Dataset, tokenizer: Any) -> datasets.Dataset:
        def _format_conversation(sample: Any) -> datasets.Dataset:
            text = tokenizer.apply_chat_template(
                sample["conversations"],
                tokenize=False,
                add_generation_prompt=False,
            )
            return {"text": text}

        print("Before standarization", dataset[randint(0, len(dataset))]["conversations"])

        dataset = standardize_sharegpt(dataset)
        format_dataset = dataset.map(_format_conversation, batched=False, keep_in_memory=True, remove_columns=list(dataset.features))

        print("After standarization", format_dataset[randint(0, len(dataset))]["text"])


        print("Starting tokenization...")
        tokenized_dataset = format_dataset.map(
            lambda sample: tokenizer(sample["text"]),
            batched=True,
            remove_columns=list(format_dataset.features),
            num_proc=8,
        )

        print("After tokenization", tokenized_dataset[randint(0, len(dataset))])

        return tokenized_dataset
    
class ResultsStore(BaseStore):
    ...