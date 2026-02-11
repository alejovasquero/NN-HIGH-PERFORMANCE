from typing_extensions import Self
import config
import json

import datasets

class DataStore:

    def load_from_hugging_face(self, dataset_path: str) -> datasets.Dataset:
        dataset = datasets.load_dataset(dataset_path)
        return dataset
    
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