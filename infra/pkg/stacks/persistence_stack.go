package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"go.uber.org/fx"
)

type PersistenceStackInput struct {
	fx.In
	Account commons.Account
}

type PersistenceStackOutput struct {
	fx.Out
	Construct   constructs.Construct     `group:"stacks"`
	DB          awsrds.CfnDBInstance     `name:"DB"`
	Credentials awssecretsmanager.Secret `name:"db_credentials"`
}

func BuildPersistenceStack(in PersistenceStackInput) PersistenceStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("PersistenceStack"),
		nil,
	)

	db := dbInstance(stack, in)
	dbCredentials := dbCredentials(stack)
	_ = credentialsAttachmentToDB(stack, db, dbCredentials)

	return PersistenceStackOutput{
		Construct:   stack,
		DB:          db,
		Credentials: dbCredentials,
	}
}

func dbCredentials(construct constructs.Construct) awssecretsmanager.Secret {
	secret := awssecretsmanager.NewSecret(
		construct,
		pointer.ToString("DBCredentials"),
		&awssecretsmanager.SecretProps{
			GenerateSecretString: &awssecretsmanager.SecretStringGenerator{
				SecretStringTemplate: pointer.ToString(`{"username": "master"}`),
				GenerateStringKey:    pointer.ToString("password"),
				PasswordLength:       pointer.ToFloat64(16),
				ExcludeCharacters:    pointer.ToString("\"@/\\"),
			},
		},
	)

	return secret
}

func credentialsAttachmentToDB(construct constructs.Construct, db awsrds.CfnDBInstance, secret awssecretsmanager.Secret) awssecretsmanager.CfnSecretTargetAttachment {
	attachment := awssecretsmanager.NewCfnSecretTargetAttachment(
		construct,
		pointer.ToString("SecretAttachmentToDB"),
		&awssecretsmanager.CfnSecretTargetAttachmentProps{
			SecretId:   secret.SecretArn(),
			TargetId:   db.Ref(),
			TargetType: pointer.ToString("AWS::RDS::DBInstance"),
		},
	)
	return attachment
}

func dbInstance(construct constructs.Construct, in PersistenceStackInput) awsrds.CfnDBInstance {
	db := awsrds.NewCfnDBInstance(
		construct,
		pointer.ToString("MetaflowDB"),
		&awsrds.CfnDBInstanceProps{
			DbName:                 pointer.ToString("metaflow"),
			AllocatedStorage:       pointer.ToString("20"),
			DbInstanceClass:        pointer.ToString("db.t3.small"),
			DeleteAutomatedBackups: pointer.ToBool(true),
			StorageType:            pointer.ToString("gp2"),
			Engine:                 pointer.ToString("postgres"),
			EngineVersion:          pointer.ToString("16.1"),
			MasterUsername:         pointer.ToString("test"),
			MasterUserPassword:     pointer.ToString("test"),
		},
	)
	return db
}
