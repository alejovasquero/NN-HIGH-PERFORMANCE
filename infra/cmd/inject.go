package main

import (
	"infra/pkg/account"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"go.uber.org/dig"
)

func StartInject() awscdk.App {
	container := dig.New()

	app := account.MainAccount()

	container.Provide(app)

	for _, stack := range account.StacksToInit() {
		container.Invoke(stack)
	}

	return app
}
