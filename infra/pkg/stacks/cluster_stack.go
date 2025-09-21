package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"go.uber.org/fx"
)

type ClusterStackInput struct {
	fx.In
	Account commons.Account
	Bucket  awss3.Bucket `name:"s3_bucket"`
}

type ClusterStackOutput struct {
	fx.Out
	Cluster     awsecs.Cluster `name:"ecs_cluster"`
	ECSTaskRole awsiam.Role    `name:"ecs_task_role"`
}

func BuildClusterStack(input ClusterStackInput) ClusterStackOutput {
	stack := awscdk.NewStack(
		input.Account.App,
		pointer.ToString("ClusterStack"),
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)

	cluster := ecsCluster(stack)
	ecsRole := buildMetadataSvcECSTaskRole(stack, input)

	return ClusterStackOutput{
		Cluster:     cluster,
		ECSTaskRole: ecsRole,
	}
}

func buildMetadataSvcECSTaskRole(stack awscdk.Stack, in ClusterStackInput) awsiam.Role {
	role := awsiam.NewRole(
		stack, pointer.ToString("MetadataSvcECSTaskRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("ecs-tasks.amazonaws.com"), nil),
			RoleName:  pointer.ToString("MetadataSvcECSTaskRole"),
		},
	)

	role.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("ObjectAccessMetadataService"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("s3:GetObject"),
				},
				Resources: &[]*string{
					pointer.ToString(*in.Bucket.ArnForObjects(pointer.ToString("*"))),
				},
			},
		),
	)

	role.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("ObjectAccessMetadataServiceNonExistentKeys"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("s3:ListBucket"),
				},
				Resources: &[]*string{
					in.Bucket.BucketArn(),
				},
			},
		),
	)

	return role
}

func ecsCluster(stack awscdk.Stack) awsecs.Cluster {
	name := "fargateECSCluster"
	return awsecs.NewCluster(
		stack,
		&name,
		&awsecs.ClusterProps{
			ContainerInsightsV2: awsecs.ContainerInsights_ENABLED,
		},
	)
}
