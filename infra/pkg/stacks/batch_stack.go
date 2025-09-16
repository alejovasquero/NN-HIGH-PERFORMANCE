package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type BatchStackInput struct {
	fx.In
	Account commons.Account
	VPC     awsec2.Vpc       `name:"metaflow_vpc"`
	SubnetA awsec2.CfnSubnet `name:"metaflow_subnet_a"`
	SubnetB awsec2.CfnSubnet `name:"metaflow_subnet_b"`
}

type BatchStackOutput struct {
	fx.Out
	Stack      awscdk.Stack                   `group:"stacks"`
	BatchRole  awsiam.Role                    `name:"batch_execution_role"`
	ComputeEnv awsbatch.CfnComputeEnvironment `name:"batch_compute_environment"`
	JobQueue   awsbatch.CfnJobQueue           `name:"batch_job_queue"`
}

func BuildBatchStack(in BatchStackInput) BatchStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("BatchStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)
	batchRole := buildBatchExecutionRole(stack)
	computeEnv := buildComputeEnvironment(stack, in, batchRole)
	jobQueue := buildJobQueue(stack, computeEnv)

	out := BatchStackOutput{
		Stack:      stack,
		BatchRole:  batchRole,
		ComputeEnv: computeEnv,
		JobQueue:   jobQueue,
	}

	return out
}

func buildComputeEnvironment(construct constructs.Construct, input BatchStackInput, batchRole awsiam.Role) awsbatch.CfnComputeEnvironment {
	computeEnv := awsbatch.NewCfnComputeEnvironment(
		construct,
		pointer.ToString("ComputeEnvironment"),
		&awsbatch.CfnComputeEnvironmentProps{
			Type:        pointer.ToString("MANAGED"),
			ServiceRole: batchRole.RoleArn(),
			ComputeResources: &awsbatch.CfnComputeEnvironment_ComputeResourcesProperty{
				Type:     pointer.ToString("FARGATE"),
				MaxvCpus: pointer.ToFloat64(16),
				SecurityGroupIds: &[]*string{
					input.VPC.VpcDefaultSecurityGroup(),
				},
				Subnets: &[]*string{
					input.SubnetA.Ref(),
					input.SubnetB.Ref(),
				},
			},
			State: pointer.ToString("ENABLED"),
		},
	)

	return computeEnv
}

func buildJobQueue(construct constructs.Construct, computeEnv awsbatch.CfnComputeEnvironment) awsbatch.CfnJobQueue {
	jobQueue := awsbatch.NewCfnJobQueue(
		construct,
		pointer.ToString("JobQueue"),
		&awsbatch.CfnJobQueueProps{
			ComputeEnvironmentOrder: &[]*awsbatch.CfnJobQueue_ComputeEnvironmentOrderProperty{
				{
					Order:              pointer.ToFloat64(1),
					ComputeEnvironment: computeEnv.Ref(),
				},
			},
			JobQueueName: pointer.ToString("MetaflowBatchJobQueue"),
			Priority:     pointer.ToFloat64(1),
			State:        pointer.ToString("ENABLED"),
		},
	)

	return jobQueue
}

func buildBatchExecutionRole(construct constructs.Construct) awsiam.Role {
	role := awsiam.NewRole(
		construct, pointer.ToString("BatchExecutionRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("batch.amazonaws.com"), nil),
			RoleName:  pointer.ToString("BatchExecutionRole"),
		},
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("VisualEditor0"),
				Actions: &[]*string{
					pointer.ToString("iam:PassRole"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Conditions: &map[string]interface{}{
					"StringEquals": map[string]interface{}{
						"iam:PassedToService": &[]*string{
							pointer.ToString("ec2.amazonaws.com"),
							pointer.ToString("ec2.amazonaws.com.cn"),
							pointer.ToString("ecs-tasks.amazonaws.com"),
						},
					},
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Sid:    pointer.ToString("VisualEditor2"),
				Actions: &[]*string{
					pointer.ToString("ec2:DescribeAccountAttributes"),
					pointer.ToString("ec2:DescribeInstances"),
					pointer.ToString("ec2:DescribeInstanceAttribute"),
					pointer.ToString("ec2:DescribeSubnets"),
					pointer.ToString("ec2:DescribeSecurityGroups"),
					pointer.ToString("ec2:DescribeKeyPairs"),
					pointer.ToString("ec2:DescribeImages"),
					pointer.ToString("ec2:DescribeImageAttribute"),
					pointer.ToString("ec2:DescribeSpotInstanceRequests"),
					pointer.ToString("ec2:DescribeSpotFleetInstances"),
					pointer.ToString("ec2:DescribeSpotFleetRequests"),
					pointer.ToString("ec2:DescribeSpotPriceHistory"),
					pointer.ToString("ec2:DescribeVpcClassicLink"),
					pointer.ToString("ec2:DescribeLaunchTemplateVersions"),
					pointer.ToString("ec2:CreateLaunchTemplate"),
					pointer.ToString("ec2:DeleteLaunchTemplate"),
					pointer.ToString("ec2:RequestSpotFleet"),
					pointer.ToString("ec2:CancelSpotFleetRequests"),
					pointer.ToString("ec2:ModifySpotFleetRequest"),
					pointer.ToString("ec2:TerminateInstances"),
					pointer.ToString("ec2:RunInstances"),
					pointer.ToString("autoscaling:DescribeAccountLimits"),
					pointer.ToString("autoscaling:DescribeAutoScalingGroups"),
					pointer.ToString("autoscaling:DescribeLaunchConfigurations"),
					pointer.ToString("autoscaling:DescribeAutoScalingInstances"),
					pointer.ToString("autoscaling:CreateLaunchConfiguration"),
					pointer.ToString("autoscaling:CreateAutoScalingGroup"),
					pointer.ToString("autoscaling:UpdateAutoScalingGroup"),
					pointer.ToString("autoscaling:SetDesiredCapacity"),
					pointer.ToString("autoscaling:DeleteLaunchConfiguration"),
					pointer.ToString("autoscaling:DeleteAutoScalingGroup"),
					pointer.ToString("autoscaling:CreateOrUpdateTags"),
					pointer.ToString("autoscaling:SuspendProcesses"),
					pointer.ToString("autoscaling:PutNotificationConfiguration"),
					pointer.ToString("autoscaling:TerminateInstanceInAutoScalingGroup"),
					pointer.ToString("ecs:DescribeClusters"),
					pointer.ToString("ecs:DescribeContainerInstances"),
					pointer.ToString("ecs:DescribeTaskDefinition"),
					pointer.ToString("ecs:DescribeTasks"),
					pointer.ToString("ecs:ListClusters"),
					pointer.ToString("ecs:ListContainerInstances"),
					pointer.ToString("ecs:ListTaskDefinitionFamilies"),
					pointer.ToString("ecs:ListTaskDefinitions"),
					pointer.ToString("ecs:ListTasks"),
					pointer.ToString("ecs:CreateCluster"),
					pointer.ToString("ecs:DeleteCluster"),
					pointer.ToString("ecs:RegisterTaskDefinition"),
					pointer.ToString("ecs:DeregisterTaskDefinition"),
					pointer.ToString("ecs:RunTask"),
					pointer.ToString("ecs:StartTask"),
					pointer.ToString("ecs:StopTask"),
					pointer.ToString("ecs:UpdateContainerAgent"),
					pointer.ToString("ecs:DeregisterContainerInstance"),
					pointer.ToString("logs:CreateLogGroup"),
					pointer.ToString("logs:CreateLogStream"),
					pointer.ToString("logs:PutLogEvents"),
					pointer.ToString("logs:DescribeLogGroups"),
					pointer.ToString("iam:GetInstanceProfile"),
					pointer.ToString("iam:GetRole"),
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
				Sid:    pointer.ToString("VisualEditor3"),
				Actions: &[]*string{
					pointer.ToString("iam:CreateServiceLinkedRole"),
					pointer.ToString("logs:PutLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Conditions: &map[string]interface{}{
					"StringEquals": map[string]any{
						"iam:AWSServiceName": &[]*string{
							pointer.ToString("autoscaling.amazonaws.com"),
							pointer.ToString("ecs.amazonaws.com"),
						},
					},
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("ec2custompolicies"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("ec2:CreateTags"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
				Conditions: &map[string]interface{}{
					"StringEquals": map[string]interface{}{
						"ec2:CreateAction": pointer.ToString("RunInstances"),
					},
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("ecr:GetAuthorizationToken"),
					pointer.ToString("ecr:BatchCheckLayerAvailability"),
					pointer.ToString("ecr:GetDownloadUrlForLayer"),
					pointer.ToString("ecr:BatchGetImage"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	return role
}
