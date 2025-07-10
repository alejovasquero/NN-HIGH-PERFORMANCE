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

type MetaflowMetadataTaskDefinitionInput struct {
	fx.In
	Account               commons.Account
	VPC                   awsec2.Vpc                               `name:"metaflow_vpc"`
	FargateSecurityGroup  awsec2.SecurityGroup                     `name:"fargate_security_group"`
	SubnetA               awsec2.Subnet                            `name:"metaflow_subnet_a"`
	SubnetB               awsec2.Subnet                            `name:"metaflow_subnet_b"`
	NLBTargetGroup        awselasticloadbalancingv2.CfnTargetGroup `name:"nlb_target_group"`
	NLBTargetGroupMigrate awselasticloadbalancingv2.CfnTargetGroup `name:"nlb_target_group_migrate"`
}

type MetaflowMetadataTaskDefinitionOutput struct {
	fx.Out
	Stack          awscdk.Stack          `group:"stacks"`
	MainDefinition awsecs.TaskDefinition `name:"main_task_definition"`
	MainService    awsecs.CfnService     `name:"main_metaflow_service"`
}

func TaskDefinitionsStack(input MetaflowMetadataTaskDefinitionInput) MetaflowMetadataTaskDefinitionOutput {
	stack := awscdk.NewStack(
		input.Account.App,
		pointer.ToString("MetaflowCoreStack"),
		nil,
	)
	mainTaskDefinition := mainTaskDefinition(stack)
	mainService := mainService(
		stack,
		input.FargateSecurityGroup,
		mainTaskDefinition,
		input.NLBTargetGroup,
		input.NLBTargetGroupMigrate,
		input.SubnetA,
		input.SubnetB)

	return MetaflowMetadataTaskDefinitionOutput{
		Stack:          stack,
		MainDefinition: mainTaskDefinition,
		MainService:    mainService,
	}
}

func mainService(
	stack awscdk.Stack,
	securityGroup awsec2.SecurityGroup,
	taskDefinition awsecs.TaskDefinition,
	nlbTarget awselasticloadbalancingv2.CfnTargetGroup,
	migrateTarget awselasticloadbalancingv2.CfnTargetGroup,
	subnets ...awsec2.Subnet) awsecs.CfnService {
	subnetsIds := make([]*string, len(subnets))

	for i, v := range subnets {
		subnetsIds[i] = v.SubnetId()
	}

	service := awsecs.NewCfnService(
		stack,
		pointer.ToString("metaflow-service-v2"),
		&awsecs.CfnServiceProps{
			LaunchType: pointer.ToString("FARGATE"),
			DeploymentConfiguration: &awsecs.CfnService_DeploymentConfigurationProperty{
				MaximumPercent:        pointer.ToFloat64(200),
				MinimumHealthyPercent: pointer.ToFloat64(100),
			},
			DesiredCount: pointer.ToFloat64(1),
			NetworkConfiguration: awsecs.CfnService_NetworkConfigurationProperty{
				AwsvpcConfiguration: awsecs.CfnService_AwsVpcConfigurationProperty{
					AssignPublicIp: pointer.ToString("ENABLED"),
					SecurityGroups: &[]*string{
						securityGroup.SecurityGroupId(),
					},
					Subnets: &subnetsIds,
				},
			},
			TaskDefinition: taskDefinition.ToString(),
			LoadBalancers: &[]awsecs.CfnService_LoadBalancerProperty{
				{
					ContainerName:  pointer.ToString("metadata-service-v2"),
					ContainerPort:  nlbTarget.Port(),
					TargetGroupArn: nlbTarget.Ref(),
				},
				{
					ContainerName:  pointer.ToString("metadata-service-v2"),
					ContainerPort:  migrateTarget.Port(),
					TargetGroupArn: migrateTarget.Ref(),
				},
			},
		},
	)
	return service
}

func mainTaskDefinition(stack awscdk.Stack) awsecs.TaskDefinition {
	task := awsecs.NewTaskDefinition(
		stack,
		pointer.ToString("Definition of main metaflow service"),
		&awsecs.TaskDefinitionProps{
			Family:        pointer.ToString("metadata-service-v2"),
			Cpu:           pointer.ToString("512"),
			MemoryMiB:     pointer.ToString("1024"),
			NetworkMode:   awsecs.NetworkMode_AWS_VPC,
			Compatibility: awsecs.Compatibility_EC2_AND_FARGATE,
		},
	)

	task.AddContainer(
		pointer.ToString("Metaflow execution container"),
		&awsecs.ContainerDefinitionOptions{
			ContainerName: pointer.ToString("metaflow-service-v2"),
			Environment: &map[string]*string{
				"MF_METADATA_DB_HOST":     pointer.ToString("TODO put rds host here"),
				"MF_METADATA_DB_PORT":     pointer.ToString("5432"),
				"MF_METADATA_DB_SSL_MODE": pointer.ToString("prefer"),
				"MF_METADATA_DB_USER":     pointer.ToString("master"),
				"MF_METADATA_DB_PSWD":     pointer.ToString("TODO PUT PASSWORD USING SECRETS MANAGER"),
			},
			Cpu:            pointer.ToFloat64(512),
			MemoryLimitMiB: pointer.ToFloat64(1024),
			Image: awsecs.AssetImage_FromRegistry(
				pointer.ToString("netflixoss/metaflow_metadata_service:v2.4.12"),
				nil,
			),
			PortMappings: &[]*awsecs.PortMapping{
				{
					ContainerPort: pointer.ToFloat64(8080),
				},
				{
					ContainerPort: pointer.ToFloat64(8082),
				},
			},
			Logging: awsecs.LogDriver_AwsLogs(
				&awsecs.AwsLogDriverProps{
					StreamPrefix: pointer.ToString("ecs"),
				},
			),
		},
	)

	return task
}
