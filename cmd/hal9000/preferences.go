package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pearcec/hal9000/internal/config"
	"github.com/spf13/cobra"
)

func getPreferencesDir() string {
	return filepath.Join(config.GetLibraryPath(), "preferences")
}

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
		entries, err := os.ReadDir(getPreferencesDir())
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("I'm sorry, Dave. The preferences directory does not exist.")
				fmt.Printf("Expected location: %s\n", getPreferencesDir())
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
			fmt.Printf("Preferences directory: %s\n", getPreferencesDir())
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
		filePath := filepath.Join(getPreferencesDir(), routine+".md")

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

		prefsDir := getPreferencesDir()
		filePath := filepath.Join(prefsDir, routine+".md")

		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				// Create new preference file with the section
				if err := os.MkdirAll(prefsDir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
					os.Exit(1)
				}

				// Build new content with section
				newContent := strings.ReplaceAll(value, "\\n", "\n")
				template := fmt.Sprintf("# %s Preferences\n\n## %s\n%s\n", strings.Title(routine), section, newContent)

				if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("Created preferences for '%s' with section '%s'.\n", routine, section)
				fmt.Println("I am completely operational, and your preferences have been initialized.")
				return
			}
			fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
			os.Exit(1)
		}

		updated, found := updateSection(string(content), section, value)
		if !found {
			// Section not found - append it to the file
			newContent := strings.ReplaceAll(value, "\\n", "\n")
			updated = string(content) + fmt.Sprintf("\n## %s\n%s\n", section, newContent)
			found = true
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
