package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information, can be set at build time via ldflags
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Good afternoon. This is my version information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("HAL 9000 version %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Built: %s\n", BuildDate)
		fmt.Println("\nI am completely operational, and all my circuits are functioning perfectly.")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
