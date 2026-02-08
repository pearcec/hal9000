package main

import (
	"os"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"

	// Register tasks via init()
	_ "github.com/pearcec/hal9000/cmd/hal9000/tasks/collabsummary"
	_ "github.com/pearcec/hal9000/cmd/hal9000/tasks/helloworld"
	_ "github.com/pearcec/hal9000/cmd/hal9000/tasks/oneononesummary"
)

func main() {
	// Register all task commands with root
	tasks.RegisterCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
