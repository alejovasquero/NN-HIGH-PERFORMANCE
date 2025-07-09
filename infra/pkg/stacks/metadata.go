package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"

	"go.uber.org/fx"
)

type MetaflowMetadataInput struct {
	fx.In
	Account commons.Account
	VPC     awsec2.Vpc    `name:"metaflow_vpc"`
	SubnetA awsec2.Subnet `name:"metaflow_subnet_a"`
	SubnetB awsec2.Subnet `name:"metaflow_subnet_b"`
}

type MetaflowMetadataOutput struct {
	fx.Out
	Stack                awscdk.Stack                              `group:"stacks"`
	ECSCluster           awsecs.Cluster                            `name:"ecs_cluster"`
	FargateSecurityGroup awsec2.SecurityGroup                      `name:"fargate_security_group"`
	LoadBalancer         awselasticloadbalancingv2.CfnLoadBalancer `name:"network_load_balancer"`
}

func BuildMetaflowMetadataStack(input MetaflowMetadataInput) MetaflowMetadataOutput {
	stack_name := "MetaflowMetadataStack"
	stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		nil,
	)

	ecsCluster := ecsCluster(stack)
	fargateSecurityGroup := fargateSecurityGroup(
		stack,
		input.VPC,
	)
	loadBalancer := loadBalancer(
		stack,
		input.VPC,
		input.SubnetA,
		input.SubnetB,
	)

	return MetaflowMetadataOutput{
		Stack:                stack,
		ECSCluster:           ecsCluster,
		FargateSecurityGroup: fargateSecurityGroup,
		LoadBalancer:         loadBalancer,
	}
}

func ecsCluster(stack awscdk.Stack) awsecs.Cluster {
	name := "fargateECSCluster"
	return awsecs.NewCluster(
		stack,
		&name,
		&awsecs.ClusterProps{
			ContainerInsightsV2: awsecs.ContainerInsights_ENABLED,
		},
	)
}

func fargateSecurityGroup(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.SecurityGroup {
	name := "FargateSecurityGroup"
	securityGroup := awsec2.NewSecurityGroup(
		stack,
		&name,
		&awsec2.SecurityGroupProps{
			Vpc: vpc,
		},
	)

	ingressRuleName := "Allow internal connections for the virtual net"
	securityGroup.AddIngressRule(
		awsec2.Peer_Ipv4(vpc.VpcCidrBlock()),
		awsec2.NewPort(
			&awsec2.PortProps{
				FromPort:             pointer.ToFloat64(8080),
				ToPort:               pointer.ToFloat64(8080),
				Protocol:             awsec2.Protocol_TCP,
				StringRepresentation: &ingressRuleName,
			},
		),
		&ingressRuleName,
		nil,
	)
	securityGroup.AddIngressRule(
		awsec2.Peer_Ipv4(vpc.VpcCidrBlock()),
		awsec2.NewPort(
			&awsec2.PortProps{
				Protocol:             awsec2.Protocol_TCP,
				FromPort:             pointer.ToFloat64(8082),
				ToPort:               pointer.ToFloat64(8082),
				StringRepresentation: &ingressRuleName,
			},
		),
		&ingressRuleName,
		nil,
	)
	securityGroup.AddIngressRule(
		securityGroup,
		awsec2.Port_AllTraffic(),
		&ingressRuleName,
		nil,
	)

	return securityGroup
}

func loadBalancer(stack awscdk.Stack, vpc awsec2.Vpc, subNets ...awsec2.Subnet) awselasticloadbalancingv2.CfnLoadBalancer {
	var subNetsIds = make([]*string, len(subNets))
	for i, v := range subNets {
		subNetsIds[i] = v.SubnetId()
	}

	loadBalancer := awselasticloadbalancingv2.NewCfnLoadBalancer(
		stack,
		pointer.ToString("Metaflow load balancer"),
		&awselasticloadbalancingv2.CfnLoadBalancerProps{
			Type:    pointer.ToString("network"),
			Subnets: &subNetsIds,
			Scheme:  pointer.ToString("internal"),
		},
	)

	associateNLBListener(
		stack,
		vpc,
		loadBalancer,
	)
	associateNLBMigrateListener(
		stack,
		vpc,
		loadBalancer,
	)

	return loadBalancer
}

func associateNLBListener(stack awscdk.Stack, vpc awsec2.Vpc, loadBalancer awselasticloadbalancingv2.CfnLoadBalancer) {
	targetGroup := awselasticloadbalancingv2.NewCfnTargetGroup(
		stack,
		pointer.ToString("NLB Main Group"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			HealthCheckIntervalSeconds: pointer.ToFloat64(10),
			HealthCheckProtocol:        pointer.ToString("TCP"),
			HealthCheckTimeoutSeconds:  pointer.ToFloat64(10),
			HealthyThresholdCount:      pointer.ToFloat64(2),
			TargetType:                 pointer.ToString("IP"),
			Protocol:                   pointer.ToString("TCP"),
			VpcId:                      vpc.VpcId(),
			UnhealthyThresholdCount:    pointer.ToFloat64(2),
			Port:                       pointer.ToFloat64(8080),
		},
	)
	_ = awselasticloadbalancingv2.NewCfnListener(
		stack,
		pointer.ToString("Main NLB Listener"),
		&awselasticloadbalancingv2.CfnListenerProps{
			Port: pointer.ToFloat64(80),
			DefaultActions: []interface{}{
				&awselasticloadbalancingv2.CfnListener_ActionProperty{
					Type:           pointer.ToString("forward"),
					TargetGroupArn: targetGroup.Ref(),
				},
			},
			Protocol:        pointer.ToString("TCP"),
			LoadBalancerArn: loadBalancer.AttrLoadBalancerArn(),
		},
	)
}

func associateNLBMigrateListener(stack awscdk.Stack, vpc awsec2.Vpc, loadBalancer awselasticloadbalancingv2.CfnLoadBalancer) {
	targetGroupMigrate := awselasticloadbalancingv2.NewCfnTargetGroup(
		stack,
		pointer.ToString("NLB Migrate Group"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			HealthCheckIntervalSeconds: pointer.ToFloat64(10),
			HealthCheckProtocol:        pointer.ToString("TCP"),
			HealthCheckTimeoutSeconds:  pointer.ToFloat64(10),
			HealthyThresholdCount:      pointer.ToFloat64(2),
			TargetType:                 pointer.ToString("IP"),
			Protocol:                   pointer.ToString("TCP"),
			VpcId:                      vpc.VpcId(),
			UnhealthyThresholdCount:    pointer.ToFloat64(2),
			Port:                       pointer.ToFloat64(8082),
		},
	)
	_ = awselasticloadbalancingv2.NewCfnListener(
		stack,
		pointer.ToString("Migrate NLB Listener"),
		&awselasticloadbalancingv2.CfnListenerProps{
			Port: pointer.ToFloat64(8082),
			DefaultActions: []interface{}{
				&awselasticloadbalancingv2.CfnListener_ActionProperty{
					Type:           pointer.ToString("forward"),
					TargetGroupArn: targetGroupMigrate.Ref(),
				},
			},
			Protocol:        pointer.ToString("TCP"),
			LoadBalancerArn: loadBalancer.AttrLoadBalancerArn(),
		},
	)
}

type MetaflowMetadataTaskDefinitionInput struct {
	fx.In
	Account commons.Account
	VPC     awsec2.Vpc    `name:"metaflow_vpc"`
	SubnetA awsec2.Subnet `name:"metaflow_subnet_a"`
	SubnetB awsec2.Subnet `name:"metaflow_subnet_b"`
}

type MetaflowMetadataTaskDefinitionOutput struct {
	fx.Out
	Stack          awscdk.Stack          `group:"stacks"`
	MainDefinition awsecs.TaskDefinition `name:"main_task_definition"`
}

func TaskDefinitionsStack(input MetaflowMetadataTaskDefinitionInput) MetaflowMetadataTaskDefinitionOutput {
	stack := awscdk.NewStack(
		input.Account.App,
		pointer.ToString("Stack for metaflow task definitions"),
		nil,
	)
	mainTaskDefinition := mainTaskDefinition(stack)

	return MetaflowMetadataTaskDefinitionOutput{
		Stack:          stack,
		MainDefinition: mainTaskDefinition,
	}
}

func mainTaskDefinition(stack awscdk.Stack) awsecs.TaskDefinition {
	task := awsecs.NewTaskDefinition(
		stack,
		pointer.ToString("Definition of main metaflow service"),
		&awsecs.TaskDefinitionProps{
			Family: pointer.ToString("metadata-service-v2"),
		},
	) // TODO finish definition
	return task
}
