package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Output HAL 9000 capabilities for Claude",
	Long:  `Outputs a summary of available CLI commands for injection into Claude sessions.`,
	Run:   runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

func runContext(cmd *cobra.Command, args []string) {
	fmt.Print(`<hal-capabilities>
## Available CLI Commands

### URL Processing
Process and save URLs to the library with AI-powered analysis:
` + "```bash" + `
hal9000 url <URL>              # Analyze URL with Claude and save to library
hal9000 url <URL> --dry-run    # Preview without saving
hal9000 url <URL> --basic      # Use basic analysis (no Claude)
hal9000 url search <term>      # Search saved URLs
` + "```" + `

### Calendar
View calendar events:
` + "```bash" + `
hal9000 calendar today         # Today's events
hal9000 calendar week          # This week's events
hal9000 calendar list --days=N # Events for N days
` + "```" + `

### Library
Read and write to the knowledge library:
` + "```bash" + `
hal9000 library read <path>    # Read entity (e.g., people-profiles/alice.md)
hal9000 library write <path>   # Write entity
hal9000 library list <type>/   # List entities by type
hal9000 library query <term>   # Search library
` + "```" + `

### Preferences
Manage task preferences:
` + "```bash" + `
hal9000 preferences list       # List all preference files
hal9000 preferences get <task> # Get preferences for a task
hal9000 preferences set <task> <section> <value>  # Update preferences
` + "```" + `

### Services
Manage background services:
` + "```bash" + `
hal9000 services status        # Check service health
hal9000 services start         # Start all enabled services
hal9000 services stop          # Stop all services
hal9000 services diagnose      # Troubleshoot service issues
hal9000 services logs          # View service logs
` + "```" + `

## Usage Notes
- Use these CLI commands to perform tasks
- For URL processing: always use ` + "`hal9000 url <URL>`" + `
- Results are saved to the library automatically
- Check preferences before tasks with ` + "`hal9000 preferences get <task>`" + `
</hal-capabilities>
`)
}
