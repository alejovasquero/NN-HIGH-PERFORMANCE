from metaflow import FlowSpec, Parameter, step
from transformers import TrainingArguments, AutoModelForSequenceClassification, Trainer, AutoTokenizer
import config

class DeepSeepR1CPUTraining(FlowSpec):
    model_config = config.DeepSeekConfig()
    
    @step
    def start(self):
        self.next(self.prep_model)

    @step
    def prep_model(self):
        print("Loading model...")
        self.tokenizer = AutoTokenizer.from_pretrained(self.model_config.model_name)
        self.training_args = TrainingArguments("deep_seek_r1_cpu")
        self.model = AutoModelForSequenceClassification.from_pretrained(pretrained_model_name_or_path=self.model_config.model_name)
        self.next(self.train_model)

    @step
    def train_model(self):
        print("Starting model fine tunning...")
        trainer = Trainer(
            self.model,
            self.training_args,
            train_dataset=[],
            eval_dataset=[],
            processing_class=self.tokenizer,
        )
        trainer.train()
        self.next(self.end)

    @step
    def end(self):
        print("Finished")

if __name__ == '__main__':
    DeepSeepR1CPUTraining()