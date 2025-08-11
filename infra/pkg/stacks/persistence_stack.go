package stacks

import (
	"github.com/AlekSi/pointer"
	"github.com/alejovasquero/NN-HIGH-PERFORMANCE/internal/commons"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
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
	Bucket      awss3.Bucket             `name:"s3_bucket"`
}

func BuildPersistenceStack(in PersistenceStackInput) PersistenceStackOutput {
	stack := awscdk.NewStack(
		in.Account.App,
		pointer.ToString("PersistenceStack"),
		&awscdk.StackProps{
			Env: in.Account.Env(),
		},
	)

	dbCredentials := dbCredentials(stack)
	db := dbInstance(stack, dbCredentials)
	_ = credentialsAttachmentToDB(stack, db, dbCredentials)
	bucket := bucket(stack)

	return PersistenceStackOutput{
		Construct:   stack,
		DB:          db,
		Credentials: dbCredentials,
		Bucket:      bucket,
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
			SecretName: awscdk.PhysicalName_GENERATE_IF_NEEDED(),
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

func dbInstance(construct constructs.Construct, credentials awssecretsmanager.Secret) awsrds.CfnDBInstance {
	usernameToken := credentials.SecretValueFromJson(pointer.ToString("username"))
	passwordToken := credentials.SecretValueFromJson(pointer.ToString("password"))

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
			EngineVersion:          awsrds.PostgresEngineVersion_VER_16_3().PostgresFullVersion(),
			MasterUsername:         usernameToken.UnsafeUnwrap(),
			MasterUserPassword:     passwordToken.UnsafeUnwrap(),
		},
	)
	return db
}

func bucket(scope constructs.Construct) awss3.Bucket {
	bucket := awss3.NewBucket(
		scope,
		pointer.ToString("MetaflowBucket"),
		&awss3.BucketProps{
			AccessControl: awss3.BucketAccessControl_PRIVATE,
			Encryption:    awss3.BucketEncryption_S3_MANAGED,
			BlockPublicAccess: awss3.NewBlockPublicAccess(
				&awss3.BlockPublicAccessOptions{
					BlockPublicAcls:       pointer.ToBool(true),
					BlockPublicPolicy:     pointer.ToBool(true),
					IgnorePublicAcls:      pointer.ToBool(true),
					RestrictPublicBuckets: pointer.ToBool(true),
				},
			),
		},
	)

	bucket.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)

	return bucket
}
