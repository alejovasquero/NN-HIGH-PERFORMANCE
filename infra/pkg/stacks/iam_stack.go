package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type IAMStackInput struct {
	fx.In
	Account commons.Account
}

type IAMStackOutput struct {
	fx.Out
	Stack             awscdk.Stack `group:"stacks"`
	MigrateLambdaRole awsiam.Role  `name:"migrate_role"`
}

func BuildIAMStack(input IAMStackInput) IAMStackOutput {
	stack := awscdk.NewStack(
		input.Account.App,
		pointer.ToString("IAMStack"),
		&awscdk.StackProps{
			Env: input.Account.Env(),
		},
	)

	return IAMStackOutput{
		Stack:             stack,
		MigrateLambdaRole: migrateLambdaRole(stack),
	}
}

func migrateLambdaRole(construct constructs.Construct) awsiam.Role {
	migrateLambdaRole := awsiam.NewRole(
		construct,
		pointer.ToString("MigrateLambdaRole"),
		&awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(pointer.ToString("lambda.amazonaws.com"), nil),
			Path:      pointer.ToString("/"),
			RoleName:  awscdk.PhysicalName_GENERATE_IF_NEEDED(),
		},
	)

	migrateLambdaRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("CreateLogGroup"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("logs:CreateLogGroup"),
					pointer.ToString("logs:CreateLogStream"),
					pointer.ToString("logs:PutLogEvents"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	migrateLambdaRole.AddToPolicy(
		awsiam.NewPolicyStatement(
			&awsiam.PolicyStatementProps{
				Sid:    pointer.ToString("VPC"),
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					pointer.ToString("ec2:CreateNetworkInterface"),
					pointer.ToString("ec2:DescribeNetworkInterfaces"),
					pointer.ToString("ec2:DeleteNetworkInterface"),
				},
				Resources: &[]*string{
					pointer.ToString("*"),
				},
			},
		),
	)

	return migrateLambdaRole
}
