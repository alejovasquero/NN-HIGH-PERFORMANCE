package stacks

import (
	"github.com/AlekSi/pointer"
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
	SubnetA           awsec2.CfnSubnet               `name:"metaflow_subnet_a"`
	SubnetB           awsec2.CfnSubnet               `name:"metaflow_subnet_b"`
}

func BuildMetaflowNetworkingStack(input MetaflowNetworkingInput) MetaflowNetworkingOutput {
	stack_name := "MetaflowNetworkingStack"

	nested_stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)

	vpc := metaflowVPC(nested_stack)
	subnetA := metaflowSubnetA(nested_stack, vpc)
	subnetB := metaflowSubnetB(nested_stack, vpc)

	iGateway := metaflowVPCInternetGateway(nested_stack)
	gatewayAttachment := internetGatewayAttachment(nested_stack, vpc, iGateway)
	routeTable, route := metaflowDefaultGateway(nested_stack, vpc, iGateway)

	subnetRouteTableAssociation(
		"subnetATableAssocitation",
		nested_stack,
		subnetA,
		routeTable,
	)
	subnetRouteTableAssociation(
		"subnetBTableAssocitation",
		nested_stack,
		subnetB,
		routeTable,
	)

	return MetaflowNetworkingOutput{
		Stack:             nested_stack,
		VPC:               vpc,
		InternetGateway:   iGateway,
		GatewayAttachment: gatewayAttachment,
		RouteTable:        routeTable,
		Route:             route,
		SubnetA:           subnetA,
		SubnetB:           subnetB,
	}
}

func metaflowVPC(stack awscdk.Stack) awsec2.Vpc {
	name := "MetaflowVPC"
	enableDNSSupport := true
	enableDNSHostName := true
	cidr := "10.20.0.0/16"

	vpc := awsec2.NewVpc(
		stack,
		&name,
		&awsec2.VpcProps{
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{},
			EnableDnsSupport:    &enableDNSSupport,
			EnableDnsHostnames:  &enableDNSHostName,
			IpAddresses:         awsec2.IpAddresses_Cidr(&cidr),
		},
	)
	return vpc
}

func metaflowSubnetA(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.CfnSubnet {
	subnetACIDR := "10.20.0.0/24"
	subnetAName := "SubnetA"
	subnetA := awsec2.NewCfnSubnet(
		stack,
		&subnetAName,
		&awsec2.CfnSubnetProps{
			VpcId:               vpc.VpcId(),
			CidrBlock:           &subnetACIDR,
			AvailabilityZone:    (*stack.AvailabilityZones())[0],
			MapPublicIpOnLaunch: pointer.ToBool(true),
			Tags: &[]*awscdk.CfnTag{
				{
					Key:   pointer.ToString("Name"),
					Value: &subnetAName,
				},
			},
		},
	)

	return subnetA
}

func metaflowSubnetB(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.CfnSubnet {
	subnetBCIDR := "10.20.1.0/24"
	subnetBName := "SubnetB"
	subnetB := awsec2.NewCfnSubnet(
		stack,
		&subnetBName,
		&awsec2.CfnSubnetProps{
			VpcId:               vpc.VpcId(),
			CidrBlock:           &subnetBCIDR,
			AvailabilityZone:    (*stack.AvailabilityZones())[1],
			MapPublicIpOnLaunch: pointer.ToBool(true),
			Tags: &[]*awscdk.CfnTag{
				{
					Key:   pointer.ToString("Name"),
					Value: &subnetBName,
				},
			},
		},
	)

	return subnetB
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
			Tags: &[]*awscdk.CfnTag{
				&awscdk.CfnTag{
					Key:   pointer.ToString("Main"),
					Value: pointer.ToString("true"),
				},
			},
		},
	)

	nameRoute := "MetaflowMainRoute"
	destinationCidrBlock := "0.0.0.0/0"
	route := awsec2.NewCfnRoute(
		stack,
		&nameRoute,
		&awsec2.CfnRouteProps{
			RouteTableId:         routeTable.Ref(),
			GatewayId:            internetGateway.Ref(),
			DestinationCidrBlock: &destinationCidrBlock,
		},
	)
	return routeTable, route
}

func subnetRouteTableAssociation(name string, stack awscdk.Stack, subnet awsec2.CfnSubnet, routeTable awsec2.CfnRouteTable) awsec2.CfnSubnetRouteTableAssociation {
	return awsec2.NewCfnSubnetRouteTableAssociation(
		stack,
		&name,
		&awsec2.CfnSubnetRouteTableAssociationProps{
			RouteTableId: routeTable.Ref(),
			SubnetId:     subnet.Ref(),
		},
	)
}
