package stacks

import (
	"fmt"
	"os"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type UIStackInput struct {
	fx.In
	Account              commons.Account
	VPC                  awsec2.Vpc               `name:"metaflow_vpc"`
	SubnetA              awsec2.CfnSubnet         `name:"metaflow_subnet_a"`
	SubnetB              awsec2.CfnSubnet         `name:"metaflow_subnet_b"`
	UISecurityGroup      awsec2.SecurityGroup     `name:"ui_security_group"`
	FargateSecurityGroup awsec2.SecurityGroup     `name:"fargate_security_group"`
	DB                   awsrds.CfnDBInstance     `name:"DB"`
	Credentials          awssecretsmanager.Secret `name:"db_credentials"`
	Bucket               awss3.Bucket             `name:"s3_bucket"`
	Cluster              awsecs.Cluster           `name:"ecs_cluster"`
	ECSTaskRole          awsiam.Role              `name:"ecs_task_role"`
}

type UIStackOutput struct {
	fx.Out
	UIStack      awscdk.Stack                              `group:"stacks"`
	LoadBalancer awselasticloadbalancingv2.CfnLoadBalancer `name:"load_balancer_ui"`
}

func BuildUIStack(in UIStackInput) UIStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("UIStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)

	loadBalancer := applicationLoadBalancer(stack, in, in.SubnetA, in.SubnetB)
	uiServiceTask := uiTaskDefinition(stack, in)
	uiStaticTask := uiStaticTaskDefinition(stack, in)

	_, listener := uiStaticService(stack, in, loadBalancer, uiStaticTask, in.SubnetA, in.SubnetB)
	_ = uiServiceFargateService(stack, in, uiServiceTask, listener, in.SubnetA, in.SubnetB)

	return UIStackOutput{
		UIStack:      stack,
		LoadBalancer: loadBalancer,
	}
}

func applicationLoadBalancer(stack awscdk.Stack, in UIStackInput, subnets ...awsec2.CfnSubnet) awselasticloadbalancingv2.CfnLoadBalancer {
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

func uiTaskDefinition(construct constructs.Construct, in UIStackInput) awsecs.TaskDefinition {
	executionRole := commons.CreateECSExecutionRole(construct, "ECS UI Role")
	task := awsecs.NewTaskDefinition(
		construct,
		pointer.ToString("Definition of ui metaflow service"),
		&awsecs.TaskDefinitionProps{
			Family:        pointer.ToString("metadata-ui-service"),
			Cpu:           pointer.ToString("512"),
			MemoryMiB:     pointer.ToString("1024"),
			NetworkMode:   awsecs.NetworkMode_AWS_VPC,
			Compatibility: awsecs.Compatibility_EC2_AND_FARGATE,
			ExecutionRole: executionRole,
			TaskRole:      in.ECSTaskRole,
		},
	)

	containerLogGroup := awslogs.NewLogGroup(
		construct,
		pointer.ToString("EcsUILogGroup"),
		&awslogs.LogGroupProps{
			LogGroupName:  pointer.ToString("ecs/metadata-ui-service"),
			Retention:     awslogs.RetentionDays_EIGHTEEN_MONTHS,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		},
	)
	containerLogGroup.GrantWrite(executionRole)

	task.AddContainer(
		pointer.ToString("Metaflow execution container"),
		&awsecs.ContainerDefinitionOptions{
			ContainerName: pointer.ToString("metadata-ui-service"),
			Environment: &map[string]*string{
				"MF_METADATA_DB_HOST":        in.DB.AttrEndpointAddress(),
				"MF_METADATA_DB_PORT":        pointer.ToString("5432"),
				"MF_METADATA_DB_SSL_MODE":    pointer.ToString("prefer"),
				"MF_METADATA_DB_NAME":        pointer.ToString("metaflow"),
				"UI_ENABLED":                 pointer.ToString("1"),
				"PATH_PREFIX":                pointer.ToString("/api/"),
				"MF_DATASTORE_ROOT":          pointer.ToString(fmt.Sprintf("s3://%s/metaflow", *in.Bucket.BucketName())),
				"METAFLOW_SERVICE_URL":       pointer.ToString("http://localhost:8083/api/metadata"),
				"METAFLOW_DEFAULT_DATASTORE": pointer.ToString("s3"),
				"METAFLOW_DEFAULT_METADATA":  pointer.ToString("service"),
			},
			Cpu:            pointer.ToFloat64(512),
			MemoryLimitMiB: pointer.ToFloat64(1024),
			Image: awsecs.AssetImage_FromRegistry(
				pointer.ToString(commons.MetaflowMetadataImage),
				nil,
			),
			Command: &[]*string{
				pointer.ToString("/opt/latest/bin/python3"),
				pointer.ToString("-m"),
				pointer.ToString("services.ui_backend_service.ui_server"),
			},
			PortMappings: &[]*awsecs.PortMapping{
				{
					ContainerPort: pointer.ToFloat64(8083),
				},
			},
			Logging: awsecs.LogDriver_AwsLogs(
				&awsecs.AwsLogDriverProps{
					StreamPrefix: pointer.ToString("ecs"),
					LogGroup:     containerLogGroup,
				},
			),
			Secrets: &map[string]awsecs.Secret{
				"MF_METADATA_DB_USER": awsecs.Secret_FromSecretsManager(in.Credentials, pointer.ToString("username")),
				"MF_METADATA_DB_PSWD": awsecs.Secret_FromSecretsManager(in.Credentials, pointer.ToString("password")),
			},
		},
	)
	in.Credentials.GrantRead(executionRole, nil)
	in.Credentials.GrantWrite(executionRole)

	return task
}

func uiStaticTaskDefinition(construct constructs.Construct, in UIStackInput) awsecs.TaskDefinition {
	executionRole := commons.CreateECSExecutionRole(construct, "ECS UI Static Role")
	task := awsecs.NewTaskDefinition(
		construct,
		pointer.ToString("Definition of ui metaflow static"),
		&awsecs.TaskDefinitionProps{
			Family:        pointer.ToString("metadata-ui-static"),
			Cpu:           pointer.ToString("512"),
			MemoryMiB:     pointer.ToString("1024"),
			NetworkMode:   awsecs.NetworkMode_AWS_VPC,
			Compatibility: awsecs.Compatibility_EC2_AND_FARGATE,
			ExecutionRole: executionRole,
			TaskRole:      in.ECSTaskRole,
		},
	)

	containerLogGroup := awslogs.NewLogGroup(
		construct,
		pointer.ToString("EcsUIStaticLogGroup"),
		&awslogs.LogGroupProps{
			LogGroupName:  pointer.ToString("ecs/metadata-ui-static"),
			Retention:     awslogs.RetentionDays_EIGHTEEN_MONTHS,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		},
	)
	containerLogGroup.GrantWrite(executionRole)

	task.AddContainer(
		pointer.ToString("Metaflow execution container"),
		&awsecs.ContainerDefinitionOptions{
			ContainerName:  pointer.ToString("metadata-ui-static"),
			Cpu:            pointer.ToFloat64(512),
			MemoryLimitMiB: pointer.ToFloat64(1024),
			Image: awsecs.AssetImage_FromRegistry(
				pointer.ToString(commons.MetaflowStaticUIImage),
				nil,
			),
			PortMappings: &[]*awsecs.PortMapping{
				{
					ContainerPort: pointer.ToFloat64(3000),
				},
			},
			Logging: awsecs.LogDriver_AwsLogs(
				&awsecs.AwsLogDriverProps{
					StreamPrefix: pointer.ToString("ecs"),
					LogGroup:     containerLogGroup,
				},
			),
		},
	)

	return task
}

func uiStaticService(
	construct constructs.Construct,
	in UIStackInput,
	loadBalancer awselasticloadbalancingv2.CfnLoadBalancer,
	taskDefinition awsecs.TaskDefinition,
	subnets ...awsec2.CfnSubnet) (awsecs.CfnService, awselasticloadbalancingv2.CfnListener) {

	subnetsIds := make([]*string, len(subnets))

	for i, v := range subnets {
		subnetsIds[i] = v.Ref()
	}

	uiTargetGroup := awselasticloadbalancingv2.NewCfnTargetGroup(
		construct,
		pointer.ToString("ALB target group ui static"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			Port:       pointer.ToFloat64(3000),
			Protocol:   pointer.ToString("HTTP"),
			TargetType: pointer.ToString("ip"),
			VpcId:      in.VPC.VpcId(),
		},
	)

	certificatePrivateKey, _ := os.ReadFile("my-private-key.pem")
	certificateBody, _ := os.ReadFile("my-certificate.pem")

	IAMcertificate := awsiam.NewCfnServerCertificate(
		construct,
		pointer.ToString("IAM Certificate"),
		&awsiam.CfnServerCertificateProps{
			CertificateBody: pointer.ToString(string(certificateBody)),
			PrivateKey:      pointer.ToString(string(certificatePrivateKey)),
		},
	)

	listener := awselasticloadbalancingv2.NewCfnListener(
		construct,
		pointer.ToString("ALB Listener ui"),
		&awselasticloadbalancingv2.CfnListenerProps{
			Port: pointer.ToFloat64(443),
			DefaultActions: &[]*awselasticloadbalancingv2.CfnListener_ActionProperty{
				{
					Type:           pointer.ToString("forward"),
					TargetGroupArn: uiTargetGroup.Ref(),
					Order:          pointer.ToFloat64(1),
				},
			},
			Protocol: pointer.ToString("HTTPS"),
			Certificates: &[]awselasticloadbalancingv2.IListenerCertificate{
				awselasticloadbalancingv2.ListenerCertificate_FromArn(IAMcertificate.AttrArn()),
			},
			LoadBalancerArn: loadBalancer.AttrLoadBalancerArn(),
		},
	)
	listener.AddDependency(IAMcertificate)

	service := awsecs.NewCfnService(
		construct,
		pointer.ToString("metadata-ui-static-service"),
		&awsecs.CfnServiceProps{
			LaunchType: pointer.ToString("FARGATE"),
			DeploymentConfiguration: &awsecs.CfnService_DeploymentConfigurationProperty{
				MaximumPercent:        pointer.ToFloat64(200),
				MinimumHealthyPercent: pointer.ToFloat64(75),
			},
			DesiredCount: pointer.ToFloat64(1),
			NetworkConfiguration: awsecs.CfnService_NetworkConfigurationProperty{
				AwsvpcConfiguration: awsecs.CfnService_AwsVpcConfigurationProperty{
					AssignPublicIp: pointer.ToString("ENABLED"),
					SecurityGroups: &[]*string{
						in.FargateSecurityGroup.SecurityGroupId(),
					},
					Subnets: &subnetsIds,
				},
			},
			TaskDefinition: taskDefinition.TaskDefinitionArn(),
			LoadBalancers: &[]awsecs.CfnService_LoadBalancerProperty{
				{
					ContainerName:  pointer.ToString("metadata-ui-static"),
					ContainerPort:  pointer.ToFloat64(3000),
					TargetGroupArn: uiTargetGroup.Ref(),
				},
			},
			Cluster: in.Cluster.ClusterArn(),
		},
	)
	service.AddDependency(listener)
	return service, listener
}

func uiServiceFargateService(
	construct constructs.Construct,
	in UIStackInput,
	taskDefinition awsecs.TaskDefinition,
	listener awselasticloadbalancingv2.CfnListener,
	subnets ...awsec2.CfnSubnet) awsecs.CfnService {

	subnetsIds := make([]*string, len(subnets))

	for i, v := range subnets {
		subnetsIds[i] = v.Ref()
	}

	uiTargetGroup := awselasticloadbalancingv2.NewCfnTargetGroup(
		construct,
		pointer.ToString("ALB target group ui"),
		&awselasticloadbalancingv2.CfnTargetGroupProps{
			Port:                       pointer.ToFloat64(8083),
			Protocol:                   pointer.ToString("HTTP"),
			TargetType:                 pointer.ToString("ip"),
			VpcId:                      in.VPC.VpcId(),
			HealthCheckPath:            pointer.ToString("/api/ping"),
			HealthCheckIntervalSeconds: pointer.ToFloat64(10),
		},
	)

	listenerRule := awselasticloadbalancingv2.NewCfnListenerRule(
		construct,
		pointer.ToString("ALB Listener Rule for UI"),
		&awselasticloadbalancingv2.CfnListenerRuleProps{
			ListenerArn: listener.Ref(),
			Priority:    pointer.ToFloat64(2),
			Actions: &[]*awselasticloadbalancingv2.CfnListenerRule_ActionProperty{
				{
					Type:           pointer.ToString("forward"),
					TargetGroupArn: uiTargetGroup.Ref(),
					Order:          pointer.ToFloat64(1),
				},
			},
			Conditions: &[]*awselasticloadbalancingv2.CfnListenerRule_RuleConditionProperty{
				{
					Field: pointer.ToString("path-pattern"),
					PathPatternConfig: &awselasticloadbalancingv2.CfnListenerRule_PathPatternConfigProperty{
						Values: &[]*string{pointer.ToString("/api/*")},
					},
				},
			},
		},
	)

	service := awsecs.NewCfnService(
		construct,
		pointer.ToString("metadata-ui-service"),
		&awsecs.CfnServiceProps{
			LaunchType: pointer.ToString("FARGATE"),
			DeploymentConfiguration: &awsecs.CfnService_DeploymentConfigurationProperty{
				MaximumPercent:        pointer.ToFloat64(200),
				MinimumHealthyPercent: pointer.ToFloat64(75),
			},
			DesiredCount: pointer.ToFloat64(1),
			NetworkConfiguration: awsecs.CfnService_NetworkConfigurationProperty{
				AwsvpcConfiguration: awsecs.CfnService_AwsVpcConfigurationProperty{
					AssignPublicIp: pointer.ToString("ENABLED"),
					SecurityGroups: &[]*string{
						in.FargateSecurityGroup.SecurityGroupId(),
					},
					Subnets: &subnetsIds,
				},
			},
			TaskDefinition: taskDefinition.TaskDefinitionArn(),
			LoadBalancers: &[]awsecs.CfnService_LoadBalancerProperty{
				{
					ContainerName:  pointer.ToString("metadata-ui-service"),
					ContainerPort:  pointer.ToFloat64(8083),
					TargetGroupArn: uiTargetGroup.Ref(),
				},
			},
			Cluster: in.Cluster.ClusterArn(),
		},
	)

	service.AddDependency(listener)
	service.AddDependency(listenerRule)
	return service
}
