package account

import (
	"infra/internal/commons"
	"infra/pkg/account/stacks"

	"github.com/aws/aws-cdk-go/awscdk/v2"
)

func MainAccount() commons.Account {

	app := awscdk.NewApp(nil)
	return app
}

func StacksToInit() map[string]any {
	return map[string]any{
		"1": stacks.MetaFlowStack,
	}
}
