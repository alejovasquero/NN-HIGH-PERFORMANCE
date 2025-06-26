package stacks

import (
	"fmt"

	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"go.uber.org/fx"
)

type MetaflowNetworkingInput struct {
	fx.In
	Account commons.Account
}

type MetaflowNetworkingOutput struct {
	fx.Out
	Stack awscdk.Stack `group:"stacks"`
	VPC   awsec2.Vpc   `name:"metaflow_vpc"`
}

func BuildMetaflowNetworkingStack(input MetaflowNetworkingInput) MetaflowNetworkingOutput {
	stack_name := "MetaflowNetworkingStack"

	nested_stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		nil,
	)
	fmt.Println("RUNNEDDDD")
	return MetaflowNetworkingOutput{
		Stack: nested_stack,
		VPC:   MetaflowVPC(nested_stack),
	}
}

func MetaflowVPC(stack awscdk.Stack) awsec2.Vpc {
	name := "MetaflowVPC"
	vpc := awsec2.NewVpc(
		stack,
		&name,
		nil,
	)
	return vpc
}
