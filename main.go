package main

import (
	"github.com/PopGreenTh/OpenLZ/cmd"
	"github.com/PopGreenTh/OpenLZ/internal/stages"
)

func main() {
	if stages.IsLambdaEnvironment() {
		stages.StartLambdaRuntime()
		return
	}
	cmd.Execute()
}
