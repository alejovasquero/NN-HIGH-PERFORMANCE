package main

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/bootstrap"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/pkg/stacks"
	"github.com/aws/jsii-runtime-go"
)

func initStacks() commons.Account {
	account := bootstrap.MainAccount()

	stacks.BuildMetaflowNetworkingStack(account)

	return account
}

func main() {
	defer jsii.Close()

	account := initStacks()

	account.App.Synth(nil)
}
