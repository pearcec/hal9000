package main

import (
	"os"

	// Register tasks via their init() functions
	_ "github.com/pearcec/hal9000/cmd/hal9000/tasks/collabsummary"
	_ "github.com/pearcec/hal9000/cmd/hal9000/tasks/onetoonesummary"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
