package stacks

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"

	"go.uber.org/fx"
)

type MetaflowMetadataInput struct {
	fx.In
	Account           commons.Account
	VPC               awsec2.Vpc           `name:"metaflow_vpc"`
	MigrateLambdaRole awsiam.Role          `name:"migrate_role"`
	SubnetA           awsec2.CfnSubnet     `name:"metaflow_subnet_a"`
	SubnetB           awsec2.CfnSubnet     `name:"metaflow_subnet_b"`
	Cluster           awsecs.Cluster       `name:"ecs_cluster"`
	UISecurityGroup   awsec2.SecurityGroup `name:"ui_security_group"` // TODO REMOVE
}

type MetaflowMetadataOutput struct {
	fx.Out
	Stack                 awscdk.Stack                              `group:"stacks"`
	LoadBalancer          awselasticloadbalancingv2.CfnLoadBalancer `name:"network_load_balancer"`
	NLBTargetGroup        awselasticloadbalancingv2.CfnTargetGroup  `name:"nlb_target_group"`
	NLBTargetGroupMigrate awselasticloadbalancingv2.CfnTargetGroup  `name:"nlb_target_group_migrate"`
	MigrateFunction       awslambda.CfnFunction                     `name:"migrate_function"`
}

func BuildMetaflowMetadataStack(input MetaflowMetadataInput) MetaflowMetadataOutput {
	stack_name := "MetaflowMetadataStack"
	stack := awscdk.NewStack(
		input.Account.App,
		&stack_name,
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)

	loadBalancer := loadBalancer(
		stack,
		input.UISecurityGroup,
		input.SubnetA,
		input.SubnetB,
	)

	nlbGroup, _ := associateNLBListener(
		stack,
		input.VPC,
		loadBalancer,
	)

	nlbGroupMigrate, _ := associateNLBMigrateListener(
		stack,
		input.VPC,
		loadBalancer,
	)

	migrateFunction := migrateFunction(stack, input, loadBalancer, input.SubnetA, input.SubnetB)

	return MetaflowMetadataOutput{
		Stack:                 stack,
		LoadBalancer:          loadBalancer,
		NLBTargetGroup:        nlbGroup,
		NLBTargetGroupMigrate: nlbGroupMigrate,
		MigrateFunction:       migrateFunction,
	}
}

func loadBalancer(stack awscdk.Stack, SecurityGroup awsec2.SecurityGroup, subNets ...awsec2.CfnSubnet) awselasticloadbalancingv2.CfnLoadBalancer {
	var subNetsIds = make([]*string, len(subNets))
	for i, v := range subNets {
		subNetsIds[i] = v.Ref()
	}

	loadBalancer := awselasticloadbalancingv2.NewCfnLoadBalancer(
		stack,
		pointer.ToString("Metaflow load balancer"),
		&awselasticloadbalancingv2.CfnLoadBalancerProps{
			Type:    pointer.ToString("network"),
			Subnets: &subNetsIds,
			Scheme:  pointer.ToString("internal"),
			SecurityGroups: &[]*string{
				SecurityGroup.SecurityGroupId(),
			},
		},
	)

	return loadBalancer
}

func associateNLBListener(stack awscdk.Stack, vpc awsec2.Vpc, loadBalancer awselasticloadbalancingv2.CfnLoadBalancer) (awselasticloadbalancingv2.CfnTargetGroup, awselasticloadbalancingv2.CfnListener) {
	targetGroup := awselasticloadbalancingv2.NewCfnTargetGroup(
		stack,
		pointer.ToString("NLB Main Group"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			HealthCheckIntervalSeconds: pointer.ToFloat64(10),
			HealthCheckProtocol:        pointer.ToString("TCP"),
			HealthCheckTimeoutSeconds:  pointer.ToFloat64(10),
			HealthyThresholdCount:      pointer.ToFloat64(2),
			TargetType:                 pointer.ToString("ip"),
			Protocol:                   pointer.ToString("TCP"),
			VpcId:                      vpc.VpcId(),
			UnhealthyThresholdCount:    pointer.ToFloat64(2),
			Port:                       pointer.ToFloat64(8080),
		},
	)
	listener := awselasticloadbalancingv2.NewCfnListener(
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

	return targetGroup, listener
}

func associateNLBMigrateListener(stack awscdk.Stack, vpc awsec2.Vpc, loadBalancer awselasticloadbalancingv2.CfnLoadBalancer) (awselasticloadbalancingv2.CfnTargetGroup, awselasticloadbalancingv2.CfnListener) {
	targetGroupMigrate := awselasticloadbalancingv2.NewCfnTargetGroup(
		stack,
		pointer.ToString("NLB Migrate Group"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			HealthCheckIntervalSeconds: pointer.ToFloat64(10),
			HealthCheckProtocol:        pointer.ToString("TCP"),
			HealthCheckTimeoutSeconds:  pointer.ToFloat64(10),
			HealthyThresholdCount:      pointer.ToFloat64(2),
			TargetType:                 pointer.ToString("ip"),
			Protocol:                   pointer.ToString("TCP"),
			VpcId:                      vpc.VpcId(),
			UnhealthyThresholdCount:    pointer.ToFloat64(2),
			Port:                       pointer.ToFloat64(8082),
		},
	)
	listener := awselasticloadbalancingv2.NewCfnListener(
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

	return targetGroupMigrate, listener
}

func migrateFunction(construct constructs.Construct, in MetaflowMetadataInput, nlbLoadBalancer awselasticloadbalancingv2.CfnLoadBalancer, subnets ...awsec2.CfnSubnet) awslambda.CfnFunction {
	subnetsIds := make([]*string, len(subnets))
	for i, v := range subnets {
		subnetsIds[i] = v.Ref()
	}

	lambda := awslambda.NewCfnFunction(
		construct,
		pointer.ToString("MigrateFunction"),
		&awslambda.CfnFunctionProps{
			Code: awslambda.CfnFunction_CodeProperty{
				ZipFile: pointer.ToString(
					`
import os, json
from urllib import request

def handler(event, context):
	response = {}
	status_endpoint = "{}/db_schema_status".format(os.environ.get('MD_LB_ADDRESS'))
	upgrade_endpoint = "{}/upgrade".format(os.environ.get('MD_LB_ADDRESS'))

	with request.urlopen(status_endpoint) as status:
		response['init-status'] = json.loads(status.read())

	upgrade_patch = request.Request(upgrade_endpoint, method='PATCH')
	with request.urlopen(upgrade_patch) as upgrade:
		response['upgrade-result'] = upgrade.read().decode()

	with request.urlopen(status_endpoint) as status:
		response['final-status'] = json.loads(status.read())

	print(response)
	return(response)
					`,
				),
			},
			Handler: pointer.ToString("index.handler"),
			Runtime: pointer.ToString("python3.9"),
			Timeout: pointer.ToFloat64(900),
			VpcConfig: &awslambda.CfnFunction_VpcConfigProperty{
				SecurityGroupIds: &[]*string{in.VPC.VpcDefaultSecurityGroup()},
				SubnetIds:        &subnetsIds,
			},
			FunctionName: pointer.ToString("metaflow-migrate"),
			Role:         in.MigrateLambdaRole.RoleArn(),
			Environment: &map[string]*string{
				"MD_LB_ADDRESS": pointer.ToString(fmt.Sprintf("http://%s:8082", *nlbLoadBalancer.AttrDnsName())),
			},
		},
	)
	return lambda
}
