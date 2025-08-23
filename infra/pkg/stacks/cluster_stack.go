package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"go.uber.org/fx"
)

type ClusterStackInput struct {
	fx.In
	Account commons.Account
}

type ClusterStackOutput struct {
	fx.Out
	Cluster awsecs.Cluster `name:"ecs_cluster"`
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

	return ClusterStackOutput{
		Cluster: cluster,
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
