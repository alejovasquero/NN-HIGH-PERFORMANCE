package bootstrap

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"

	"github.com/aws/aws-cdk-go/awscdk/v2"
)

func MainAccount() commons.Account {
	app := awscdk.NewApp(nil)
	return commons.Account{
		App:       app,
		AccountId: "450119683363",
		Region:    "us-east-2",
	}
}
