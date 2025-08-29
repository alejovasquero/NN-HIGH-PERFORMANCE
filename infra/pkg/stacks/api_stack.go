package stacks

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type ApiStackInput struct {
	fx.In
	Account      commons.Account
	LoadBalancer awselasticloadbalancingv2.CfnLoadBalancer `name:"network_load_balancer"`
}

type ApiStackOutput struct {
	fx.Out
	Stack      awscdk.Stack          `group:"stacks"`
	ApiGateway awsapigateway.RestApi `name:"api_gateway"`
}

func BuildApiStack(input ApiStackInput) ApiStackOutput {
	stack := awscdk.NewStack(
		input.Account.App,
		pointer.ToString("ApiStack"),
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)

	apiGateway := apiGateway(stack, input)

	return ApiStackOutput{
		Stack:      stack,
		ApiGateway: apiGateway,
	}
}

func apiGateway(construct constructs.Construct, input ApiStackInput) awsapigateway.RestApi {
	api := awsapigateway.NewRestApi(
		construct,
		pointer.ToString("ApiGateway"),
		nil,
	)

	root := api.Root()

	proxy := root.AddResource(
		pointer.ToString("{proxy+}"),
		nil,
	)

	vpcLink := vpcLink(construct, input)

	api.Node().AddDependency(vpcLink)

	integration := awsapigateway.NewIntegration(
		&awsapigateway.IntegrationProps{
			Type:                  awsapigateway.IntegrationType_HTTP_PROXY,
			IntegrationHttpMethod: pointer.ToString("ANY"),
			Uri:                   pointer.ToString(fmt.Sprintf("http://%s/{proxy}", *input.LoadBalancer.AttrDnsName())),
			Options: &awsapigateway.IntegrationOptions{
				ConnectionType: awsapigateway.ConnectionType_VPC_LINK,
				VpcLink:        vpcLink,
				CacheKeyParameters: &[]*string{
					pointer.ToString("method.request.path.proxy"),
				},
				RequestParameters: &map[string]*string{
					"integration.request.path.proxy": pointer.ToString("method.request.path.proxy"),
				},
				PassthroughBehavior: awsapigateway.PassthroughBehavior_WHEN_NO_MATCH,
				IntegrationResponses: &[]*awsapigateway.IntegrationResponse{
					{
						StatusCode: pointer.ToString("200"),
					},
				},
			},
		},
	)

	proxy.AddMethod(
		pointer.ToString("ANY"),
		integration,
		&awsapigateway.MethodOptions{
			ApiKeyRequired:    pointer.ToBool(false),
			AuthorizationType: awsapigateway.AuthorizationType_NONE,
			RequestParameters: &map[string]*bool{
				"method.request.path.proxy": pointer.ToBool(true),
			},
		},
	)

	dbResource := root.AddResource(
		pointer.ToString("db_schema_status"),
		nil,
	)

	dbIntegration := awsapigateway.NewIntegration(
		&awsapigateway.IntegrationProps{
			Type:                  awsapigateway.IntegrationType_HTTP_PROXY,
			IntegrationHttpMethod: pointer.ToString("GET"),
			Uri:                   pointer.ToString(fmt.Sprintf("http://%s:8082/db_schema_status", *input.LoadBalancer.AttrDnsName())),
			Options: &awsapigateway.IntegrationOptions{
				PassthroughBehavior: awsapigateway.PassthroughBehavior_WHEN_NO_MATCH,
				IntegrationResponses: &[]*awsapigateway.IntegrationResponse{
					{
						StatusCode: pointer.ToString("200"),
					},
				},
				VpcLink: vpcLink,
			},
		},
	)

	dbResource.AddMethod(
		pointer.ToString("GET"),
		dbIntegration,
		&awsapigateway.MethodOptions{
			ApiKeyRequired:    pointer.ToBool(false),
			AuthorizationType: awsapigateway.AuthorizationType_NONE,
		},
	)

	deployment := awsapigateway.NewDeployment(
		construct,
		pointer.ToString("APIDeployment"),
		&awsapigateway.DeploymentProps{
			Api: api,
		},
	)

	awsapigateway.NewStage(
		construct,
		pointer.ToString("APIStage"),
		&awsapigateway.StageProps{
			StageName:  pointer.ToString("api"),
			Deployment: deployment,
		},
	)

	return api
}

func vpcLink(construct constructs.Construct, input ApiStackInput) awsapigateway.VpcLink {
	networkLoadBalancer := awselasticloadbalancingv2.NetworkLoadBalancer_FromLookup(
		construct,
		pointer.ToString("NetworkLoadBalancer"),
		&awselasticloadbalancingv2.NetworkLoadBalancerLookupOptions{
			LoadBalancerArn: input.LoadBalancer.AttrLoadBalancerArn(),
		},
	)

	vpcLink := awsapigateway.NewVpcLink(
		construct,
		pointer.ToString("VpcLink"),
		&awsapigateway.VpcLinkProps{
			Targets: &[]awselasticloadbalancingv2.INetworkLoadBalancer{
				networkLoadBalancer,
			},
		},
	)

	return vpcLink
}
