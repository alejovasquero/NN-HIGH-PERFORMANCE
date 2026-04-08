# Infra for metaflow in AWS using CDK

## Prerequisites
- AWS CLI
- AWS Credentials
- Docker
- GO
- AWS user with permissions to assume cdk roles

You also can build this folder in devcontainers. Easier!!

## Deploy to AWS
```
go run cmd/cobra/main.go deploy
```


## List metaflow config in AWS
```
go run cmd/cobra/main.go metaflow-config
```

## Destroy all AWS resources
```
go run cmd/cobra/main.go destroy
```

Note:
- When running metaflow make sure your user has assigned the MetaflowPolicy for running flows
