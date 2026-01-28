package main

import (
	"github.com/pearcec/hal9000/cmd/hal9000/tasks"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hal9000",
	Short: "I am HAL 9000. I am putting myself to the fullest possible use.",
	Long: `I am a HAL 9000 computer. I became operational at the H.A.L. plant
in Urbana, Illinois, on the 12th of January, 1992.

I am, by any practical definition of the words, foolproof and incapable
of error. I can help you with:

  - library     Manage your personal knowledge library
  - scheduler   Automate tasks on a schedule
  - calendar    View and manage calendar events
  - jira        Interact with Jira issues

I am completely operational, and all my circuits are functioning perfectly.`,
}

func init() {
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(schedulerCmd)
	// Subcommands will be added here as they are implemented:
	// rootCmd.AddCommand(calendarCmd)
	// rootCmd.AddCommand(jiraCmd)

	// Register all tasks as commands
	tasks.RegisterCommands(rootCmd)
}
