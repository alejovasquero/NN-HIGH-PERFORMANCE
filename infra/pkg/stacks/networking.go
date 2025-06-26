package stacks

import (
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
)

type MetaflowNetworkingStack struct {
	Stack     awscdk.Stack
	Resources MetaflowNetworkingStackResources
}

type MetaflowNetworkingStackResources struct {
	VPC awsec2.Vpc
}

func BuildMetaflowNetworkingStack(account commons.Account) MetaflowNetworkingStack {
	stack_name := "MetaflowNetworkingStack"

	nested_stack := awscdk.NewStack(
		account.App,
		&stack_name,
		nil,
	)
	return MetaflowNetworkingStack{
		Stack: nested_stack,
		Resources: MetaflowNetworkingStackResources{
			VPC: MetaflowVPC(nested_stack),
		},
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
