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
	Stack             awscdk.Stack                   `group:"stacks"`
	VPC               awsec2.Vpc                     `name:"metaflow_vpc"`
	InternetGateway   awsec2.CfnInternetGateway      `name:"metaflow_internet_gateway"`
	GatewayAttachment awsec2.CfnVPCGatewayAttachment `name:"metaflow_gateway_attachment"`
	RouteTable        awsec2.CfnRouteTable           `name:"metaflow_route_table"`
	Route             awsec2.CfnRoute                `name:"metaflow_route"`
}

func BuildMetaflowNetworkingStack(input MetaflowNetworkingInput) MetaflowNetworkingOutput {
	stack_name := "MetaflowNetworkingStack"

	nested_stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		nil,
	)

	vpc := metaflowVPC(nested_stack)
	iGateway := metaflowVPCInternetGateway(nested_stack)
	gatewayAttachment := internetGatewayAttachment(nested_stack, vpc, iGateway)
	routeTable, route := metaflowDefaultGateway(nested_stack, vpc, iGateway)

	return MetaflowNetworkingOutput{
		Stack:             nested_stack,
		VPC:               vpc,
		InternetGateway:   iGateway,
		GatewayAttachment: gatewayAttachment,
		RouteTable:        routeTable,
		Route:             route,
	}
}

func metaflowVPC(stack awscdk.Stack) awsec2.Vpc {
	name := "MetaflowVPC"
	enableDNSSupport := true
	enableDNSHostName := true
	vpc := awsec2.NewVpc(
		stack,
		&name,
		&awsec2.VpcProps{
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{},
			EnableDnsSupport:    &enableDNSSupport,
			EnableDnsHostnames:  &enableDNSHostName,
		},
	)
	return vpc
}

func metaflowVPCInternetGateway(stack awscdk.Stack) awsec2.CfnInternetGateway {
	name := "MetaflowVPCInternetGateway"
	i_gateway := awsec2.NewCfnInternetGateway(
		stack,
		&name,
		&awsec2.CfnInternetGatewayProps{},
	)
	return i_gateway
}

func internetGatewayAttachment(stack awscdk.Stack, vpc awsec2.Vpc, internetGateway awsec2.CfnInternetGateway) awsec2.CfnVPCGatewayAttachment {
	name := "MetaflowInternetGatewayAttachment"
	internet_attachment := awsec2.NewCfnVPCGatewayAttachment(
		stack,
		&name,
		&awsec2.CfnVPCGatewayAttachmentProps{
			VpcId:             vpc.VpcId(),
			InternetGatewayId: internetGateway.Ref(),
		},
	)
	return internet_attachment
}

func metaflowDefaultGateway(stack awscdk.Stack, vpc awsec2.Vpc, internetGateway awsec2.CfnInternetGateway) (awsec2.CfnRouteTable, awsec2.CfnRoute) {
	name := "MetaflowRouteTable"

	routeTable := awsec2.NewCfnRouteTable(
		stack,
		&name,
		&awsec2.CfnRouteTableProps{
			VpcId: vpc.VpcId(),
		},
	)
	fmt.Println("HELLO")
	nameRoute := "MetaflowMainRoute"
	route := awsec2.NewCfnRoute(
		stack,
		&nameRoute,
		&awsec2.CfnRouteProps{
			RouteTableId: routeTable.Ref(),
			GatewayId:    internetGateway.Ref(),
		},
	)
	return routeTable, route
}
