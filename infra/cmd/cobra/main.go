package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"

	"github.com/AlekSi/pointer"
	"github.com/spf13/cobra"
)

const a string = `
{
    "METAFLOW_BATCH_JOB_QUEUE": "arn:aws:batch:us-east-2:450119683363:job-queue/MetaflowBatchJobQueue",
    "METAFLOW_DATASTORE_SYSROOT_S3": "s3://persistencestack-metaflowbucketb2a7a044-5av20e1o10ur/metaflow",
    "METAFLOW_DATATOOLS_S3ROOT": "s3://persistencestack-metaflowbucketb2a7a044-5av20e1o10ur/data",
    "METAFLOW_DEFAULT_DATASTORE": "s3",
    "METAFLOW_DEFAULT_METADATA": "service",
    "METAFLOW_ECS_S3_ACCESS_IAM_ROLE": "arn:aws:iam::450119683363:role/BatchS3Role",
    "METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE": "arn:aws:iam::450119683363:role/EventBridgeRole",
    "METAFLOW_SERVICE_INTERNAL_URL": "https://Metafl-Metaf-foadUCUjiZzO-338cd3c42ad22e9b.elb.us-east-2.amazonaws.com/",
    "METAFLOW_SERVICE_URL": "https://5kedgbj7ol.execute-api.us-east-2.amazonaws.com/api/",
    "METAFLOW_SFN_DYNAMO_DB_TABLE": "MetaflowStepFunctionsState",
    "METAFLOW_SFN_IAM_ROLE": "arn:aws:iam::450119683363:role/StepFunctionsRole",
    "METAFLOW_ECS_FARGATE_EXECUTION_ROLE": "arn:aws:iam::450119683363:role/BatchExecutionRole"
}`

const figlet string = `    _   ___   ____  ______________  ______  __________  __________  ____  __  ______    _   ______________
   / | / / | / / / / /  _/ ____/ / / / __ \/ ____/ __ \/ ____/ __ \/ __ \/  |/  /   |  / | / / ____/ ____/
  /  |/ /  |/ / /_/ // // / __/ /_/ / /_/ / __/ / /_/ / /_  / / / / /_/ / /|_/ / /| | /  |/ / /   / __/   
 / /|  / /|  / __  // // /_/ / __  / ____/ /___/ _, _/ __/ / /_/ / _, _/ /  / / ___ |/ /|  / /___/ /___   
/_/ |_/_/ |_/_/ /_/___/\____/_/ /_/_/   /_____/_/ |_/_/    \____/_/ |_/_/  /_/_/  |_/_/ |_/\____/_____/   										  
`

type MetaflowConfig struct {
	METAFLOW_BATCH_JOB_QUEUE            *string `json:"METAFLOW_BATCH_JOB_QUEUE"`
	METAFLOW_DATASTORE_SYSROOT_S3       *string `json:"METAFLOW_DATASTORE_SYSROOT_S3"`
	METAFLOW_DATATOOLS_S3ROOT           *string `json:"METAFLOW_DATATOOLS_S3ROOT"`
	METAFLOW_DEFAULT_DATASTORE          *string `json:"METAFLOW_DEFAULT_DATASTORE"`
	METAFLOW_DEFAULT_METADATA           *string `json:"METAFLOW_DEFAULT_METADATA"`
	METAFLOW_ECS_S3_ACCESS_IAM_ROLE     *string `json:"METAFLOW_ECS_S3_ACCESS_IAM_ROLE"`
	METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE *string `json:"METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE"`
	METAFLOW_SERVICE_INTERNAL_URL       *string `json:"METAFLOW_SERVICE_INTERNAL_URL"`
	METAFLOW_SERVICE_URL                *string `json:"METAFLOW_SERVICE_URL"`
	METAFLOW_SFN_DYNAMO_DB_TABLE        *string `json:"METAFLOW_SFN_DYNAMO_DB_TABLE"`
	METAFLOW_SFN_IAM_ROLE               *string `json:"METAFLOW_SFN_IAM_ROLE"`
	METAFLOW_ECS_FARGATE_EXECUTION_ROLE *string `json:"METAFLOW_ECS_FARGATE_EXECUTION_ROLE"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "app",
		Short: "app application entry point",
	}

	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the CDK application",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(figlet)
			execCmd := exec.Command("cdk", "deploy", "--all", "--require-approval", "never", "--concurrency", "100")
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Run()
		},
	}

	destroyCmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the CDK application",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(figlet)
			execCmd := exec.Command("cdk", "destroy", "--all", "--force", "--concurrency", "100")
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Run()
		},
	}

	metaflowConfigCmd := &cobra.Command{
		Use:   "metaflow-config",
		Short: "Show the Metaflow configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(figlet)
			cfnCommand := exec.Command("aws", "cloudformation", "describe-stacks", "--stack-name", "ResultStack", "--query", "Stacks[0].Outputs[][Description, OutputValue]")
			result, err := cfnCommand.Output()

			if err != nil {
				fmt.Println("Error executing command:", err)
				return
			}

			var resulsList [][]string

			err = json.Unmarshal(result, &resulsList)
			if err != nil {
				fmt.Println("Error parsing JSON:", err)
				return
			}

			config := &MetaflowConfig{
				METAFLOW_DEFAULT_METADATA:  pointer.ToString("service"),
				METAFLOW_DEFAULT_DATASTORE: pointer.ToString("s3"),
			}
			typeVar := reflect.ValueOf(config).Elem()

			for _, item := range resulsList {
				name := item[0]
				value := item[1]
				if field := typeVar.FieldByName(name); field.IsValid() {
					field.Set(reflect.ValueOf(&value))
				}
			}

			bytes, err := json.MarshalIndent(config, "", "  ")

			if err != nil {
				fmt.Println("Error parsing JSON:", err)
				return
			}

			fmt.Println(string(bytes))
			fmt.Println("Configuration for ~/.metaflowconfig/config.json")
		},
	}

	rootCmd.AddCommand(deployCmd, destroyCmd, metaflowConfigCmd)

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
