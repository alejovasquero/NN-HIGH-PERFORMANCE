package stacks

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type RolesStackInput struct {
	fx.In
	Account        commons.Account
	MetaflowBucket awss3.Bucket               `name:"s3_bucket"`
	JobQueue       awsbatch.CfnJobQueue       `name:"batch_job_queue"`
	StateDDB       awsdynamodb.CfnGlobalTable `name:"state_ddb"`
}

type RolesStackOutput struct {
	fx.Out
	Stack              awscdk.Stack         `group:"stacks"`
	MetaflowUserRole   awsiam.Role          `name:"metaflow_user_role"`
	EventBridgeRole    awsiam.Role          `name:"event_bridge_role"`
	StepFunctionsRole  awsiam.Role          `name:"step_functions_role"`
	BatchS3Role        awsiam.Role          `name:"batch_s3_role"`
	MetaflowUserPolicy awsiam.ManagedPolicy `name:"metaflow_user_policy"`
}

func BuildRolesStack(in RolesStackInput) RolesStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("RolesStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)
	metaflowUserRole := buildMetaflowUserRole(stack, in)
	eventBridgeRole := buildEventBridgeRole(stack, in)
	stepFunctionsRole := buildStepFunctionsRole(stack, in)
	batchS3Role := buildBatchS3Role(stack, in)
	metaflowUserPolicy := buildMetaflowUserPolicy(stack, in)

	out := RolesStackOutput{
		Stack:              stack,
		MetaflowUserRole:   metaflowUserRole,
		EventBridgeRole:    eventBridgeRole,
		StepFunctionsRole:  stepFunctionsRole,
		BatchS3Role:        batchS3Role,
		MetaflowUserPolicy: metaflowUserPolicy,
	}

	return out
}

func buildMetaflowUserRole(construct constructs.Construct, input RolesStackInput) awsiam.Role {
	ecsExecutionRole := commons.CreateECSExecutionRole(construct, "ECSExecutionRoleMetaflowUser")
	role := awsiam.NewRole(
		construct, pointer.ToString("MetaflowUserRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewArnPrincipal(ecsExecutionRole.RoleArn()),
			RoleName:  pointer.ToString("MetaflowUserRole"),
			Path:      pointer.ToString("/"),
		})

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("cloudformation:DescribeStacks"),
					pointer.ToString("cloudformation:*Stack"),
					pointer.ToString("cloudformation:*ChangeSet"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:cloudformation:%[1]s:%[2]s:stack/*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("s3:*Object"),
				},
				Resources: &[]*string{
					input.MetaflowBucket.ArnForObjects(pointer.ToString("*")),
					input.MetaflowBucket.BucketArn(),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("sagemaker:DescribeNotebook*"),
					pointer.ToString("sagemaker:StartNotebookInstance"),
					pointer.ToString("sagemaker:StopNotebookInstance"),
					pointer.ToString("sagemaker:UpdateNotebookInstance"),
					pointer.ToString("sagemaker:CreatePresignedNotebookInstanceUrl"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:notebook-instance/*", input.Account.Region, input.Account.AccountId)),
					pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:notebook-instance-lifecycle-config/basic*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("iam:PassRole"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:iam::%[1]s:role/*", input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("kms:Decrypt"),
					pointer.ToString("kms:Encrypt"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:kms:%[1]s:%[2]s:key/", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("JobsPermissions"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("batch:TerminateJob"),
					pointer.ToString("batch:DescribeJobs"),
					pointer.ToString("batch:DescribeJobDefinitions"),
					pointer.ToString("batch:DescribeJobQueues"),
					pointer.ToString("batch:RegisterJobDefinition"),
					pointer.ToString("batch:DescribeComputeEnvironments"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("DefinitionsPermissions"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("batch:SubmitJob"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:batch:%[1]s:%[2]s:job-definition/*:*", input.Account.Region, input.Account.AccountId)),
					input.JobQueue.Ref(),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("BucketAccess"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("s3:ListBucket"),
				},
				Resources: &[]*string{
					input.MetaflowBucket.BucketArn(),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("GetLogs"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("logs:GetLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:logs:%[1]s:%[2]s:log-group:*:log-stream:*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("AllowSagemakerCreate"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("sagemaker:CreateTrainingJob"),
					pointer.ToString("sagemaker:DescribeTrainingJob"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:training-job/*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("TasksAndExecutionsGlobal"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("states:ListStateMachines"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("StateMachines"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("states:DescribeStateMachine"),
					pointer.ToString("states:UpdateStateMachine"),
					pointer.ToString("states:StartExecution"),
					pointer.ToString("states:CreateStateMachine"),
					pointer.ToString("states:ListExecutions"),
					pointer.ToString("states:StopExecution"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:states:%[1]s:%[2]s:stateMachine:*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("VisualEditor0"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("events:PutTargets"),
					pointer.ToString("events:DisableRule"),
					pointer.ToString("events:PutRule"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:events:%[1]s:%[2]s:rule/*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	return role
}

func buildEventBridgeRole(construct constructs.Construct, input RolesStackInput) awsiam.Role {
	role := awsiam.NewRole(
		construct, pointer.ToString("EventBridgeRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("events.amazonaws.com"), nil),
			RoleName:  pointer.ToString("EventBridgeRole"),
		},
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("ExecuteStepFunction"),
				Actions: &[]*string{
					pointer.ToString("states:StartExecution"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:states:%[1]s:%[2]s:stateMachine:*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	return role
}

func buildStepFunctionsRole(construct constructs.Construct, input RolesStackInput) awsiam.Role {
	role := awsiam.NewRole(
		construct, pointer.ToString("StepFunctionsRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("states.amazonaws.com"), nil),
			RoleName:  pointer.ToString("StepFunctionsRole"),
		},
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("JobsPermissions"),
				Actions: &[]*string{
					pointer.ToString("batch:TerminateJob"),
					pointer.ToString("batch:DescribeJobs"),
					pointer.ToString("batch:DescribeJobDefinitions"),
					pointer.ToString("batch:DescribeJobQueues"),
					pointer.ToString("batch:RegisterJobDefinition"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("DefinitionsPermissions"),
				Actions: &[]*string{
					pointer.ToString("batch:SubmitJob"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:batch:%[1]s:%[2]s:job-definition/*:*", input.Account.Region, input.Account.AccountId)),
					input.JobQueue.Ref(),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("BucketAccess"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("s3:ListBucket"),
					pointer.ToString("s3:*Object"),
				},
				Resources: &[]*string{
					input.MetaflowBucket.BucketArn(),
					input.MetaflowBucket.ArnForObjects(pointer.ToString("*")),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("CloudwatchLogDelivery"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("logs:CreateLogDelivery"),
					pointer.ToString("logs:GetLogDelivery"),
					pointer.ToString("logs:UpdateLogDelivery"),
					pointer.ToString("logs:DeleteLogDelivery"),
					pointer.ToString("logs:ListLogDeliveries"),
					pointer.ToString("logs:PutResourcePolicy"),
					pointer.ToString("logs:DescribeResourcePolicies"),
					pointer.ToString("logs:DescribeLogGroups"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("RuleMaintenance"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("events:PutTargets"),
					pointer.ToString("events:DescribeRule"),
					pointer.ToString("events:PutRule"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:events:%[1]s:%[2]s:rule/StepFunctionsGetEventsForBatchJobsRule", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)
	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("Items"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("dynamodb:PutItem"),
					pointer.ToString("dynamodb:GetItem"),
					pointer.ToString("dynamodb:UpdateItem"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:dynamodb:%[1]s:%[2]s:table/%[3]s", input.Account.Region, input.Account.AccountId, *input.StateDDB.TableName())),
				},
			},
		),
	)

	return role
}

func buildBatchS3Role(construct constructs.Construct, input RolesStackInput) awsiam.Role {
	role := awsiam.NewRole(
		construct, pointer.ToString("BatchS3Role"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("ecs-tasks.amazonaws.com"), nil),
			RoleName:  pointer.ToString("BatchS3Role"),
		},
	)
	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("BucketAccessBatch"),
				Actions: &[]*string{
					pointer.ToString("s3:ListBucket"),
				},
				Resources: &[]*string{
					input.MetaflowBucket.BucketArn(),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("ObjectAccessBatch"),
				Actions: &[]*string{
					pointer.ToString("s3:PutObject"),
					pointer.ToString("s3:GetObject"),
					pointer.ToString("s3:DeleteObject"),
				},
				Resources: &[]*string{
					input.MetaflowBucket.ArnForObjects(pointer.ToString("*")),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_DENY,
				Sid:    pointer.ToString("DenyPresignedBatch"),
				Actions: &[]*string{
					pointer.ToString("s3:*"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Conditions: &map[string]interface{}{
					"StringNotEquals": map[string]*string{
						"s3:authType": pointer.ToString("REST-HEADER"),
					},
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("AllowSagemaker"),
				Actions: &[]*string{
					pointer.ToString("sagemaker:CreateTrainingJob"),
					pointer.ToString("sagemaker:DescribeTrainingJob"),
					pointer.ToString("sagemaker:CreateModel"),
					pointer.ToString("sagemaker:CreateEndpointConfig"),
					pointer.ToString("sagemaker:CreateEndpoint"),
					pointer.ToString("sagemaker:DescribeModel"),
					pointer.ToString("sagemaker:DescribeEndpoint"),
					pointer.ToString("sagemaker:InvokeEndpoint"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:batch:%[1]s:%[2]s:*", input.Account.Region, input.Account.AccountId)),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("AllowPassRole"),
				Actions: &[]*string{
					pointer.ToString("iam:PassRole"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Conditions: &map[string]interface{}{
					"StringEquals": map[string]*string{
						"iam:PassedToService": pointer.ToString("sagemaker.amazonaws.com"),
					},
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("Items"),
				Actions: &[]*string{
					pointer.ToString("dynamodb:PutItem"),
					pointer.ToString("dynamodb:GetItem"),
					pointer.ToString("dynamodb:UpdateItem"),
				},
				Resources: &[]*string{
					pointer.ToString(fmt.Sprintf("arn:aws:dynamodb:%[1]s:%[2]s:table/%[3]s", input.Account.Region, input.Account.AccountId, *input.StateDDB.TableName())),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("AllowPutLogs"),
				Actions: &[]*string{
					pointer.ToString("logs:CreateLogStream"),
					pointer.ToString("logs:PutLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	return role
}

func buildMetaflowUserPolicy(construct constructs.Construct, input RolesStackInput) awsiam.ManagedPolicy {
	policy := awsiam.NewManagedPolicy(
		construct,
		pointer.ToString("MetaflowUserPolicy"),
		&awsiam.ManagedPolicyProps{
			ManagedPolicyName: pointer.ToString("MetaflowUserPolicy"),
			Statements: &[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("cloudformation:DescribeStacks"),
							pointer.ToString("cloudformation:*Stack"),
							pointer.ToString("cloudformation:*ChangeSet"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:cloudformation:%[1]s:%[2]s:stack/*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("s3:*"),
						},
						Resources: &[]*string{
							input.MetaflowBucket.ArnForObjects(pointer.ToString("*")),
							input.MetaflowBucket.BucketArn(),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("sagemaker:DescribeNotebook*"),
							pointer.ToString("sagemaker:StartNotebookInstance"),
							pointer.ToString("sagemaker:StopNotebookInstance"),
							pointer.ToString("sagemaker:UpdateNotebookInstance"),
							pointer.ToString("sagemaker:CreatePresignedNotebookInstanceUrl"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:notebook-instance/*", input.Account.Region, input.Account.AccountId)),
							pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:notebook-instance-lifecycle-config/basic*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("iam:PassRole"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:iam::%[1]s:role/*", input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("kms:Decrypt"),
							pointer.ToString("kms:Encrypt"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:kms:%[1]s:%[2]s:key/", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("JobsPermissions"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("batch:TerminateJob"),
							pointer.ToString("batch:DescribeJobs"),
							pointer.ToString("batch:DescribeJobDefinitions"),
							pointer.ToString("batch:DescribeJobQueues"),
							pointer.ToString("batch:RegisterJobDefinition"),
							pointer.ToString("batch:DescribeComputeEnvironments"),
						},
						Resources: &[]*string{
							pointer.ToString("*"),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("DefinitionsPermissions"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("batch:SubmitJob"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:batch:%[1]s:%[2]s:job-definition/*:*", input.Account.Region, input.Account.AccountId)),
							input.JobQueue.Ref(),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("GetLogs"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("logs:GetLogEvents"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:logs:%[1]s:%[2]s:log-group:*:log-stream:*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("AllowSagemakerCreate"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("sagemaker:CreateTrainingJob"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:training-job/*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("AllowSagemakerDescribe"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("sagemaker:DescribeTrainingJob"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:sagemaker:%[1]s:%[2]s:training-job/*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("TasksAndExecutionsGlobal"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("states:ListStateMachines"),
						},
						Resources: &[]*string{
							pointer.ToString("*"),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("StateMachines"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("states:DescribeStateMachine"),
							pointer.ToString("states:UpdateStateMachine"),
							pointer.ToString("states:StartExecution"),
							pointer.ToString("states:CreateStateMachine"),
							pointer.ToString("states:ListExecutions"),
							pointer.ToString("states:StopExecution"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:states:%[1]s:%[2]s:stateMachine:*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("RuleMaintenance"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("events:PutTargets"),
							pointer.ToString("events:DisableRule"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:events:%[1]s:%[2]s:rule/*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
				awsiam.NewPolicyStatement(
					&awsiam.PolicyStatementProps{
						Sid:    pointer.ToString("PutRule"),
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							pointer.ToString("events:PutRule"),
						},
						Resources: &[]*string{
							pointer.ToString(fmt.Sprintf("arn:aws:events:%[1]s:%[2]s:rule/*", input.Account.Region, input.Account.AccountId)),
						},
					},
				),
			},
		},
	)

	policy.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN_ON_UPDATE_OR_DELETE)
	return policy
}
