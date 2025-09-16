package stacks

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssagemaker"
	"go.uber.org/fx"
)

type ResultStackInput struct {
	fx.In
	Account           commons.Account
	MetaflowBucket    awss3.Bucket                              `name:"s3_bucket"`
	JobQueue          awsbatch.CfnJobQueue                      `name:"batch_job_queue"`
	ApiGateway        awsapigateway.RestApi                     `name:"api_gateway"`
	BatchS3Role       awsiam.Role                               `name:"batch_s3_role"`
	LoadBalancer      awselasticloadbalancingv2.CfnLoadBalancer `name:"network_load_balancer"`
	NotebookInstance  awssagemaker.CfnNotebookInstance          `name:"sagemaker_notebook_instance"`
	EventBridgeRole   awsiam.Role                               `name:"event_bridge_role"`
	StepFunctionsRole awsiam.Role                               `name:"step_functions_role"`
	StateDDB          awsdynamodb.CfnGlobalTable                `name:"state_ddb"`
}

type ResultStackOutput struct {
	fx.Out
	Stack                awscdk.Stack     `group:"stacks"`
	MetaflowDataStoreURL awscdk.CfnOutput `name:"sysroot_s3"`
	MetaflowDataToolsURL awscdk.CfnOutput `name:"datatools_s3"`
	BatchJobQueue        awscdk.CfnOutput `name:"batch_job_queue_name"`
	ServiceURL           awscdk.CfnOutput `name:"service_url"`
	RoleForJobs          awscdk.CfnOutput `name:"role_for_jobs"`
	InternalServiceURL   awscdk.CfnOutput `name:"internal_service_url"`
	NotebooksURL         awscdk.CfnOutput `name:"notebooks_url"`
	EventBridgeRoleARN   awscdk.CfnOutput `name:"event_bridge_role_arn"`
	StepFunctionRoleARN  awscdk.CfnOutput `name:"step_functions_role_arn"`
	StepFunctionsDDBARN  awscdk.CfnOutput `name:"step_functions_ddb_arn"`
}

func BuildResultStack(in ResultStackInput) ResultStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("ResultStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)

	metaflowDataStoreURL := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_DATASTORE_SYSROOT_S3"),
		&awscdk.CfnOutputProps{
			Value:       pointer.ToString(fmt.Sprintf("s3://%s/metaflow", *in.MetaflowBucket.BucketName())),
			Description: pointer.ToString("METAFLOW_DATASTORE_SYSROOT_S3"),
		},
	)

	metaflowDataToolsURL := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_DATATOOLS_S3ROOT"),
		&awscdk.CfnOutputProps{
			Value:       pointer.ToString(fmt.Sprintf("s3://%s/data", *in.MetaflowBucket.BucketName())),
			Description: pointer.ToString("METAFLOW_DATATOOLS_S3ROOT"),
		},
	)

	metaflowJobQueue := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_BATCH_JOB_QUEUE"),
		&awscdk.CfnOutputProps{
			Value:       in.JobQueue.Ref(),
			Description: pointer.ToString("METAFLOW_BATCH_JOB_QUEUE"),
		},
	)

	serviceURL := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_SERVICE_URL"),
		&awscdk.CfnOutputProps{
			Value:       pointer.ToString(fmt.Sprintf("https://%[1]s.execute-api.%[2]s.amazonaws.com/api/", *in.ApiGateway.RestApiId(), in.Account.Region)),
			Description: pointer.ToString("METAFLOW_SERVICE_URL"),
		},
	)

	roleForJobs := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_ECS_S3_ACCESS_IAM_ROLE"),
		&awscdk.CfnOutputProps{
			Value:       in.BatchS3Role.RoleArn(),
			Description: pointer.ToString("METAFLOW_ECS_S3_ACCESS_IAM_ROLE"),
		},
	)

	internalServiceURL := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_SERVICE_INTERNAL_URL"),
		&awscdk.CfnOutputProps{
			Value:       pointer.ToString(fmt.Sprintf("https://%s/", *in.LoadBalancer.AttrDnsName())),
			Description: pointer.ToString("METAFLOW_SERVICE_INTERNAL_URL"),
		},
	)

	notebookURL := awscdk.NewCfnOutput(
		stack, pointer.ToString("NOTEBOOKS_URL"),
		&awscdk.CfnOutputProps{
			Value:       pointer.ToString(fmt.Sprintf("https://%s.notebook.%s.sagemaker.aws/tree", *in.NotebookInstance.NotebookInstanceName(), in.Account.Region)),
			Description: pointer.ToString("NOTEBOOKS_URL"),
		},
	)

	eventBridgeRole := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE"),
		&awscdk.CfnOutputProps{
			Value:       in.EventBridgeRole.RoleArn(),
			Description: pointer.ToString("METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE"),
		},
	)

	stepFunctionsRole := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_SFN_IAM_ROLE"),
		&awscdk.CfnOutputProps{
			Value:       in.StepFunctionsRole.RoleArn(),
			Description: pointer.ToString("METAFLOW_SFN_IAM_ROLE"),
		},
	)

	stepFunctionsDDBARN := awscdk.NewCfnOutput(
		stack, pointer.ToString("METAFLOW_SFN_DYNAMO_DB_TABLE"),
		&awscdk.CfnOutputProps{
			Value:       in.StateDDB.TableName(),
			Description: pointer.ToString("METAFLOW_SFN_DYNAMO_DB_TABLE"),
		},
	)

	out := ResultStackOutput{
		Stack:                stack,
		MetaflowDataStoreURL: metaflowDataStoreURL,
		MetaflowDataToolsURL: metaflowDataToolsURL,
		BatchJobQueue:        metaflowJobQueue,
		ServiceURL:           serviceURL,
		RoleForJobs:          roleForJobs,
		InternalServiceURL:   internalServiceURL,
		NotebooksURL:         notebookURL,
		EventBridgeRoleARN:   eventBridgeRole,
		StepFunctionRoleARN:  stepFunctionsRole,
		StepFunctionsDDBARN:  stepFunctionsDDBARN,
	}

	return out
}
