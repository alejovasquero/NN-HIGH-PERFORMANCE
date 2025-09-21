package commons

import (
	"github.com/AlekSi/pointer"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
)

func CreateECSExecutionRole(construct constructs.Construct, name string) awsiam.Role {
	executionRole := awsiam.NewRole(
		construct,
		pointer.ToString(name),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("ecs-tasks.amazonaws.com"), nil),
			Path:      pointer.ToString("/"),
			RoleName:  awscdk.PhysicalName_GENERATE_IF_NEEDED(),
		},
	)

	executionRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("ecr:GetAuthorizationToken"),
					pointer.ToString("ecr:BatchCheckLayerAvailability"),
					pointer.ToString("ecr:GetDownloadUrlForLayer"),
					pointer.ToString("ecr:BatchGetImage"),
					pointer.ToString("logs:CreateLogStream"),
					pointer.ToString("logs:PutLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	return executionRole
}
