package commons

import (
	"github.com/AlekSi/pointer"
	"github.com/aws/aws-cdk-go/awscdk/v2"
)

type Account struct {
	App       awscdk.App
	AccountId string
}

func (a *Account) Env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: &a.AccountId,
		Region:  pointer.ToString("us-east-2"),
	}
}

type IStack interface {
	GetStack() awscdk.Stack
	GetName() string
}

type Stack struct {
	Name  string
	Stack awscdk.Stack
}

func (s Stack) GetStack() awscdk.Stack {
	return s.Stack
}

func (s Stack) GetName() string {
	return s.Name
}

type IResource interface {
	GetResource() awscdk.Resource
}

type Resource struct {
	Resource awscdk.Resource
}

func (r Resource) GetResource() awscdk.Resource {
	return r.Resource
}
