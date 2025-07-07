package stacks

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"go.uber.org/fx"
)

type MetaflowMetadataInput struct {
	fx.In
	Account commons.Account
	VPC     awsec2.Vpc `name:"metaflow_vpc"`
}

type MetaflowMetadataOutput struct {
	fx.Out
	Stack                awscdk.Stack         `group:"stacks"`
	ECSCluster           awsecs.Cluster       `name:"ecs_cluster"`
	FargateSecurityGroup awsec2.SecurityGroup `name:"fargate_security_group"`
}

func BuildMetaflowMetadataStack(input MetaflowMetadataInput) MetaflowMetadataOutput {
	stack_name := "MetaflowMetadataStack"
	stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		nil,
	)

	ecsCluster := ecsCluster(stack)
	fargateSecurityGroup := fargateSecurityGroup(
		stack,
		input.VPC,
	)

	return MetaflowMetadataOutput{
		Stack:                stack,
		ECSCluster:           ecsCluster,
		FargateSecurityGroup: fargateSecurityGroup,
	}
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

func fargateSecurityGroup(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.SecurityGroup {
	name := "FargateSecurityGroup"
	securityGroup := awsec2.NewSecurityGroup(
		stack,
		&name,
		&awsec2.SecurityGroupProps{
			Vpc: vpc,
		},
	)
	return securityGroup
}
