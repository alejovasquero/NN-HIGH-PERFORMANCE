package stacks

import (
	"fmt"

	"encoding/base64"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssagemaker"
	"github.com/aws/constructs-go/constructs/v10"

	"go.uber.org/fx"
)

type NotebookStackInput struct {
	fx.In
	Account           commons.Account
	SubnetA           awsec2.CfnSubnet                          `name:"metaflow_subnet_a"`
	VPC               awsec2.Vpc                                `name:"metaflow_vpc"`
	LoadBalancer      awselasticloadbalancingv2.CfnLoadBalancer `name:"network_load_balancer"`
	Bucket            awss3.Bucket                              `name:"s3_bucket"`
	JobQueue          awsbatch.CfnJobQueue                      `name:"batch_job_queue"`
	BatchS3Role       awsiam.Role                               `name:"batch_s3_role"`
	EventBridgeRole   awsiam.Role                               `name:"event_bridge_role"`
	StateDDB          awsdynamodb.CfnGlobalTable                `name:"state_ddb"`
	StepFunctionsRole awsiam.Role                               `name:"step_functions_role"`
	BatchRole         awsiam.Role                               `name:"batch_execution_role"`
}

type NotebookStackOutput struct {
	fx.Out
	Stack                  awscdk.Stack                     `group:"stacks"`
	NotebookInstance       awssagemaker.CfnNotebookInstance `name:"sagemaker_notebook_instance"`
	SageMakerExecutionRole awsiam.Role                      `name:"sagemaker_execution_role"`
	SageMakerSecurityGroup awsec2.SecurityGroup             `name:"sagemaker_security_group"`
}

func BuildNotebooksStack(in NotebookStackInput) NotebookStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("NotebookStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)

	notebookExecutionRole := buildSageMakerExecutionRole(stack)
	securityGroup := buildSageMakerSecurityGroup(stack, in)
	notebookLifecycleConfig := buildNotebookLyfecycle(stack, in)

	notebookinstance := buildSageMakerInstance(stack, in, notebookExecutionRole, securityGroup, notebookLifecycleConfig)

	out := NotebookStackOutput{
		Stack:                  stack,
		NotebookInstance:       notebookinstance,
		SageMakerExecutionRole: notebookExecutionRole,
		SageMakerSecurityGroup: securityGroup,
	}

	return out
}

func buildSageMakerInstance(scope constructs.Construct, input NotebookStackInput, executionRole awsiam.Role, securityGroup awsec2.SecurityGroup, lifecycleConfig awssagemaker.CfnNotebookInstanceLifecycleConfig) awssagemaker.CfnNotebookInstance {
	notebookInstance := awssagemaker.NewCfnNotebookInstance(
		scope,
		pointer.ToString("NoteBookInstance"),
		&awssagemaker.CfnNotebookInstanceProps{
			NotebookInstanceName: pointer.ToString("NotebookNNHighPerformance"),
			InstanceType:         pointer.ToString("ml.t3.large"),
			RoleArn:              executionRole.RoleArn(),
			LifecycleConfigName:  lifecycleConfig.AttrNotebookInstanceLifecycleConfigName(),
			SecurityGroupIds: &[]*string{
				securityGroup.SecurityGroupId(),
			},
			SubnetId: input.SubnetA.AttrSubnetId(),
		},
	)

	return notebookInstance
}

func buildNotebookLyfecycle(scope constructs.Construct, input NotebookStackInput) awssagemaker.CfnNotebookInstanceLifecycleConfig {

	createHook := fmt.Sprintf(
		`#!/bin/bash
echo 'export METAFLOW_DATASTORE_SYSROOT_S3=s3://%[1]s/metaflow/' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_DATATOOLS_S3ROOT=s3://%[1]s/data/' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_SERVICE_URL=http://%[2]s/' >> /etc/profile.d/jupyter-env.sh
echo 'export AWS_DEFAULT_REGION=%[3]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_DEFAULT_DATASTORE=s3' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_DEFAULT_METADATA=service' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_BATCH_JOB_QUEUE=%[4]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_ECS_S3_ACCESS_IAM_ROLE=%[5]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_EVENTS_SFN_ACCESS_IAM_ROLE=%[6]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_SFN_DYNAMO_DB_TABLE=%[7]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_SFN_IAM_ROLE=%[8]s' >> /etc/profile.d/jupyter-env.sh
echo 'export METAFLOW_ECS_FARGATE_EXECUTION_ROLE=%[9]s' >> /etc/profile.d/jupyter-env.sh
echo -e "Finished create script"
systemctl restart jupyter-server`,
		*input.Bucket.BucketName(),
		*input.LoadBalancer.AttrDnsName(),
		input.Account.Region,
		*input.JobQueue.AttrJobQueueArn(),
		*input.BatchS3Role.RoleArn(),
		*input.EventBridgeRole.RoleArn(),
		*input.StateDDB.TableName(),
		*input.StepFunctionsRole.RoleArn(),
		*input.BatchRole.RoleArn(),
	)

	startHook := `#!/bin/bash
set -e
sudo -u ec2-user -i <<'EOF'
echo "THIS IS A PLACE HOLDER TO EXECUTE - USER LEVEL" >> ~/.customrc
EOF`

	config := awssagemaker.NewCfnNotebookInstanceLifecycleConfig(
		scope,
		pointer.ToString("NotebookLifeCycle"),
		&awssagemaker.CfnNotebookInstanceLifecycleConfigProps{
			OnCreate: &[]interface{}{
				awssagemaker.CfnNotebookInstanceLifecycleConfig_NotebookInstanceLifecycleHookProperty{
					Content: awscdk.Fn_Base64(&createHook),
				},
			},
			OnStart: &[]any{
				&awssagemaker.CfnNotebookInstanceLifecycleConfig_NotebookInstanceLifecycleHookProperty{
					Content: pointer.ToString(base64.StdEncoding.EncodeToString([]byte(startHook))),
				},
			},
		},
	)

	return config
}

func buildSageMakerExecutionRole(scope constructs.Construct) awsiam.Role {
	executionRole := awsiam.NewRole(
		scope,
		pointer.ToString("SageMakerExecutionRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(
				pointer.ToString("sagemaker.amazonaws.com"),
				nil,
			),
			Path:     pointer.ToString("/"),
			RoleName: awscdk.PhysicalName_GENERATE_IF_NEEDED(),
		},
	)

	executionRole.AddToPolicy(
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

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("MiscPermissions"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("cloudwatch:PutMetricData"),
					pointer.ToString("ecr:GetDownloadUrlForLayer"),
					pointer.ToString("ecr:BatchGetImage"),
					pointer.ToString("ecr:GetAuthorizationToken"),
					pointer.ToString("ecr:BatchCheckLayerAvailability"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("CreateLogStream"),
				Actions: &[]*string{
					pointer.ToString("logs:CreateLogStream"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_ALLOW,
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("LogEvents"),
				Actions: &[]*string{
					pointer.ToString("logs:PutLogEvents"),
					pointer.ToString("logs:GetLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_ALLOW,
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("LogGroup"),
				Actions: &[]*string{
					pointer.ToString("logs:DescribeLogGroups"),
					pointer.ToString("logs:DescribeLogStreams"),
					pointer.ToString("logs:CreateLogGroup"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_ALLOW,
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("SageMakerNotebook"),
				Actions: &[]*string{
					pointer.ToString("sagemaker:DescribeNotebook*"),
					pointer.ToString("sagemaker:StartNotebookInstance*"),
					pointer.ToString("sagemaker:StopNotebookInstance*"),
					pointer.ToString("sagemaker:UpdateNotebookInstance*"),
					pointer.ToString("sagemaker:CreatePresignedNotebookInstanceUrl*"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_ALLOW,
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("BucketAccess"),
				Actions: &[]*string{
					pointer.ToString("s3:ListBucket"),
					pointer.ToString("s3:PutObject"),
					pointer.ToString("s3:GetObject"),
					pointer.ToString("s3:DeleteObject"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_ALLOW,
			},
		),
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid: pointer.ToString("DenyPresigned"),
				Actions: &[]*string{
					pointer.ToString("s3:*"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Effect: awsiam.Effect_DENY,
				Conditions: &map[string]interface{}{
					"StringNotEquals": map[string]*string{
						"s3:authType": pointer.ToString("REST-HEADER"),
					},
				},
			},
		),
	)

	return executionRole
}

func buildSageMakerSecurityGroup(scope constructs.Construct, input NotebookStackInput) awsec2.SecurityGroup {
	group := awsec2.NewSecurityGroup(
		scope,
		pointer.ToString("SageMakerSecurityGroup"),
		&awsec2.SecurityGroupProps{
			Description: pointer.ToString("Security group for sagemaker"),
			Vpc:         input.VPC,
		},
	)

	group.AddIngressRule(
		awsec2.Peer_Ipv4(pointer.ToString("0.0.0.0/0")),
		awsec2.NewPort(
			&awsec2.PortProps{
				Protocol:             awsec2.Protocol_TCP,
				FromPort:             pointer.ToFloat64(8080),
				ToPort:               pointer.ToFloat64(8080),
				StringRepresentation: pointer.ToString("Internet Access"),
			},
		),
		pointer.ToString("Allow internet access in 8080"),
		nil,
	)

	return group
}
