package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type UIStackInput struct {
	fx.In
	Account         commons.Account
	SubnetA         awsec2.CfnSubnet     `name:"metaflow_subnet_a"`
	SubnetB         awsec2.CfnSubnet     `name:"metaflow_subnet_b"`
	UISecurityGroup awsec2.SecurityGroup `name:"ui_security_group"`
}

type UIStackOutput struct {
	fx.Out
	UIStack constructs.Construct
}

func BuildUIStack(in UIStackInput) UIStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("UIStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)

	return UIStackOutput{
		UIStack: stack,
	}
}

func applicationLoadBalancer(stack awscdk.Stack, vpc awsec2.Vpc, subnets ...awsec2.CfnSubnet, in UIStackInput) awsec2.CfnLoadBalancer {
	var subnetsIds = make([]*string, len(subnets))

	for i, v := range subnets {
		subnetsIds[i] = v.Ref()
	}

	loadBalancer := awselasticloadbalancingv2.NewCfnLoadBalancer(
		stack,
		pointer.ToString("LoadBalancerUI"),
		&awselasticloadbalancingv2.CfnLoadBalancerProps{
			Subnets:        &subnetsIds,
			Type:           pointer.ToString("application"),
			SecurityGroups: &[]*string{in.UISecurityGroup.SecurityGroupId()},
		},
	)

	return loadBalancer
}
