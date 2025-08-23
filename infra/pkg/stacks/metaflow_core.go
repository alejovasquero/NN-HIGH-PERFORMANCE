package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"go.uber.org/fx"
)

type MetaflowMetadataTaskDefinitionInput struct {
	fx.In
	Account               commons.Account
	VPC                   awsec2.Vpc                               `name:"metaflow_vpc"`
	ECSCluster            awsecs.Cluster                           `name:"ecs_cluster"`
	FargateSecurityGroup  awsec2.SecurityGroup                     `name:"fargate_security_group"`
	SubnetA               awsec2.CfnSubnet                         `name:"metaflow_subnet_a"`
	SubnetB               awsec2.CfnSubnet                         `name:"metaflow_subnet_b"`
	NLBTargetGroup        awselasticloadbalancingv2.CfnTargetGroup `name:"nlb_target_group"`
	NLBTargetGroupMigrate awselasticloadbalancingv2.CfnTargetGroup `name:"nlb_target_group_migrate"`
	DB                    awsrds.CfnDBInstance                     `name:"DB"`
	Credentials           awssecretsmanager.Secret                 `name:"db_credentials"`
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
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)
	mainTaskDefinition := mainTaskDefinition(stack, input)
	mainService := mainService(
		stack,
		input.FargateSecurityGroup,
		mainTaskDefinition,
		input.NLBTargetGroup,
		input.ECSCluster,
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
	cluster awsecs.Cluster,
	migrateTarget awselasticloadbalancingv2.CfnTargetGroup,
	subnets ...awsec2.CfnSubnet) awsecs.CfnService {

	subnetsIds := make([]*string, len(subnets))

	for i, v := range subnets {
		subnetsIds[i] = v.Ref()
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
			TaskDefinition: taskDefinition.TaskDefinitionArn(),
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
			Cluster: cluster.ClusterArn(),
		},
	)
	return service
}

func mainTaskDefinition(stack awscdk.Stack, input MetaflowMetadataTaskDefinitionInput) awsecs.TaskDefinition {
	executionRole := commons.CreateECSExecutionRole(stack, "ECS Service Role")
	task := awsecs.NewTaskDefinition(
		stack,
		pointer.ToString("Definition of main metaflow service"),
		&awsecs.TaskDefinitionProps{
			Family:        pointer.ToString("metadata-service-v2"),
			Cpu:           pointer.ToString("512"),
			MemoryMiB:     pointer.ToString("1024"),
			NetworkMode:   awsecs.NetworkMode_AWS_VPC,
			Compatibility: awsecs.Compatibility_EC2_AND_FARGATE,
			ExecutionRole: executionRole,
		},
	)

	containerLogGroup := awslogs.NewLogGroup(
		stack,
		pointer.ToString("EcsLogGroup"),
		&awslogs.LogGroupProps{
			LogGroupName:  pointer.ToString("ecs/metadata-service-v2"),
			Retention:     awslogs.RetentionDays_EIGHTEEN_MONTHS,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		},
	)

	containerLogGroup.GrantWrite(executionRole)

	task.AddContainer(
		pointer.ToString("Metaflow execution container"),
		&awsecs.ContainerDefinitionOptions{
			ContainerName: pointer.ToString("metadata-service-v2"),
			Environment: &map[string]*string{
				"MF_METADATA_DB_HOST":     input.DB.AttrEndpointAddress(),
				"MF_METADATA_DB_PORT":     pointer.ToString("5432"),
				"MF_METADATA_DB_SSL_MODE": pointer.ToString("prefer"),
				"MF_METADATA_DB_NAME":     pointer.ToString("metaflow"),
			},
			Cpu:            pointer.ToFloat64(512),
			MemoryLimitMiB: pointer.ToFloat64(1024),
			Image: awsecs.AssetImage_FromRegistry(
				pointer.ToString(commons.MetaflowMetadataImage),
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
					LogGroup:     containerLogGroup,
				},
			),
			Secrets: &map[string]awsecs.Secret{
				"MF_METADATA_DB_USER": awsecs.Secret_FromSecretsManager(input.Credentials, pointer.ToString("username")),
				"MF_METADATA_DB_PSWD": awsecs.Secret_FromSecretsManager(input.Credentials, pointer.ToString("password")),
			},
		},
	)
	input.Credentials.GrantRead(executionRole, nil)
	input.Credentials.GrantWrite(executionRole)

	return task
}
