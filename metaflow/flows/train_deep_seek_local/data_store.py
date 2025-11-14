from typing_extensions import Self
import config
import json

import datasets

class DataStore:

    def load_from_hugging_face(cls, datastore_config: config.DataStoreConfig) -> Self:
        dataset = datasets.load_dataset(path=datastore_config.hugging_face_name)
        dataset.save_to_disk(datastore_config.local_path)
        return cls()
    

    def _format_conversation(self, conversation: list) -> datasets.Dataset:
        human_prompt = conversation[0]
        gpt_response = conversation[1]
        return json.dumps(
                {
                    "messages": [
                        {
                            "role": "user",
                            "content": human_prompt["value"]
                        },
                        {
                            "role": "gpt",
                            "content": gpt_response["value"]
                        }
                    ]
                }
        )