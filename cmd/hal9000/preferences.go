package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var preferencesDir = filepath.Join(os.Getenv("HOME"), "Documents", "Google Drive", "Claude", "preferences")

var preferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "Manage routine preferences",
	Long: `I can help you manage your routine preferences.

Preferences are stored as markdown files in your preferences directory,
organized by routine (e.g., agenda, calendar, etc.).

Available subcommands:
  list    - List all preference files
  get     - Get preferences for a routine
  set     - Update a preference section`,
}

var preferencesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all preference files",
	Long:  `I will enumerate all available preference files for you.`,
	Run: func(cmd *cobra.Command, args []string) {
		entries, err := os.ReadDir(preferencesDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("I'm sorry, Dave. The preferences directory does not exist.")
				fmt.Printf("Expected location: %s\n", preferencesDir)
				return
			}
			fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
			os.Exit(1)
		}

		var prefs []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				name := strings.TrimSuffix(entry.Name(), ".md")
				prefs = append(prefs, name)
			}
		}

		if len(prefs) == 0 {
			fmt.Println("No preference files found.")
			fmt.Printf("Preferences directory: %s\n", preferencesDir)
			return
		}

		fmt.Println("Available preferences:")
		for _, p := range prefs {
			fmt.Printf("  - %s\n", p)
		}
	},
}

var preferencesGetCmd = &cobra.Command{
	Use:   "get <routine>",
	Short: "Get preferences for a routine",
	Long:  `I will retrieve the preferences for the specified routine.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		routine := args[0]
		filePath := filepath.Join(preferencesDir, routine+".md")

		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("I'm sorry, Dave. I cannot find preferences for '%s'.\n", routine)
				fmt.Println("Use 'hal9000 preferences list' to see available preferences.")
				return
			}
			fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(content))
	},
}

var preferencesSetCmd = &cobra.Command{
	Use:   "set <routine> <section> <value>",
	Short: "Update a preference section",
	Long: `I will update a specific section in your preferences file.

The section should match a markdown header (e.g., "Priority Rules", "Format").
The value will replace the content under that section header.

Example:
  hal9000 preferences set agenda "Notes" "- Keep it brief\n- Focus on top 3"`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		routine := args[0]
		section := args[1]
		value := args[2]

		filePath := filepath.Join(preferencesDir, routine+".md")

		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("I'm sorry, Dave. I cannot find preferences for '%s'.\n", routine)
				fmt.Println("Use 'hal9000 preferences list' to see available preferences.")
				return
			}
			fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
			os.Exit(1)
		}

		updated, found := updateSection(string(content), section, value)
		if !found {
			fmt.Printf("I'm sorry, Dave. I cannot find section '%s' in %s preferences.\n", section, routine)
			fmt.Println("Use 'hal9000 preferences get <routine>' to see available sections.")
			return
		}

		err = os.WriteFile(filePath, []byte(updated), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Updated section '%s' in %s preferences.\n", section, routine)
		fmt.Println("I am completely operational, and your preferences have been modified.")
	},
}

// updateSection finds a markdown section by header and replaces its content.
// It handles ## headers and replaces content until the next ## header.
func updateSection(content, sectionName, newValue string) (string, bool) {
	lines := strings.Split(content, "\n")
	var result []string
	found := false
	inTargetSection := false
	headerLevel := 0

	for _, line := range lines {
		// Check if this is a header line
		if strings.HasPrefix(line, "##") {
			// Count the header level
			level := 0
			for _, c := range line {
				if c == '#' {
					level++
				} else {
					break
				}
			}

			// Extract header text (after ## and space)
			headerText := strings.TrimSpace(strings.TrimLeft(line, "#"))

			if strings.EqualFold(headerText, sectionName) {
				// Found our target section
				found = true
				inTargetSection = true
				headerLevel = level
				result = append(result, line)
				result = append(result, "")
				// Add new content (handle escaped newlines)
				newContent := strings.ReplaceAll(newValue, "\\n", "\n")
				result = append(result, newContent)
				continue
			} else if inTargetSection && level <= headerLevel {
				// Found next section at same or higher level, stop skipping
				inTargetSection = false
			}
		}

		if !inTargetSection {
			result = append(result, line)
		}
	}

	if !found {
		return content, false
	}

	return strings.Join(result, "\n"), true
}

func init() {
	preferencesCmd.AddCommand(preferencesListCmd)
	preferencesCmd.AddCommand(preferencesGetCmd)
	preferencesCmd.AddCommand(preferencesSetCmd)
	rootCmd.AddCommand(preferencesCmd)
}
