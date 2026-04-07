from typing import Any
import os
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
        dataset = datasets.load_dataset(dataset_path, split="train[:1000]")
        return dataset

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