package bootstrap

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/stacks"

	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"

	"github.com/aws/aws-cdk-go/awscdk/v2"
)

func MainAccount() commons.Account {
	app := awscdk.NewApp(nil)
	return commons.Account{
		App: app,
	}
}

func StacksToInit() map[string]any {
	return map[string]any{
		"1": stacks.MetaFlowStack,
	}
}
