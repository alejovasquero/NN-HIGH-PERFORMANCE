package main

import (

	// "github.com/aws/aws-cdk-go/awscdk/v2/awssqs"

	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()

	app := StartInject()

	app.Synth(nil)
}
