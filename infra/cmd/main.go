package main

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/bootstrap"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/stacks"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"go.uber.org/fx"
)

type StacksInput struct {
	fx.In
	Shuwdownser fx.Shutdowner
	Stacks      []awscdk.Stack `group:"stacks"`
}

func initStacks() commons.Account {
	account := bootstrap.MainAccount()

	container := fx.New(
		fx.Supply(account),
		fx.Provide(stacks.BuildMetaflowNetworkingStack),
		fx.Provide(stacks.BuildMetaflowMetadataStack),
		fx.Provide(stacks.TaskDefinitionsStack),
		fx.Provide(stacks.BuildPersistenceStack),
		fx.Provide(stacks.BuildClusterStack),
		fx.Provide(stacks.BuildUIStack),
		fx.Provide(stacks.BuildIAMStack),
		fx.Invoke(func(input StacksInput) int {
			input.Shuwdownser.Shutdown()
			return 0
		}),
	)

	container.Run()
	return account
}

func main() {
	defer jsii.Close()

	account := initStacks()

	account.App.Synth(nil)
}
