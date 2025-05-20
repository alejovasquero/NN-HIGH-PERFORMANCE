package main

import (
	"log"

	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/bootstrap"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/stacks"
	"github.com/aws/jsii-runtime-go"
	"go.uber.org/dig"
)

func initStacks() commons.Account {
	container := dig.New()

	account := bootstrap.MainAccount()

	err := container.Provide(func() commons.Account { return account })

	if err != nil {
		log.Panic(err)
	}

	stacks.MetaFlowStack(account)

	return account
}

func main() {
	defer jsii.Close()

	account := initStacks()

	account.App.Synth(nil)
}
