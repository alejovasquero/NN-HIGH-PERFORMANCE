package stacks

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
)

func MetaFlowStack(scope commons.Account) awscdk.Stack {
	id := "MetaFlowStack"
	return awscdk.NewStack(
		scope.App,
		&id,
		&awscdk.StackProps{},
	)
}
