package stacks

import (
	"infra/internal/commons"

	"github.com/aws/aws-cdk-go/awscdk/v2"
)

func MetaFlowStack(scope commons.Account) awscdk.Stack {
	id := "MetaFlowStack"
	return awscdk.NewStack(
		scope,
		&id,
		nil,
	)
}
