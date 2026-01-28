package tasks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultPreferencesDir is the default location for preferences files.
var DefaultPreferencesDir = filepath.Join(os.Getenv("HOME"), "Documents", "Google Drive", "Claude", "preferences")

// PreferencesExist checks if preferences exist for a task.
func PreferencesExist(task Task) bool {
	return PreferencesExistIn(task, DefaultPreferencesDir)
}

// PreferencesExistIn checks if preferences exist for a task in a specific directory.
func PreferencesExistIn(task Task, dir string) bool {
	path := filepath.Join(dir, task.PreferencesKey()+".md")
	_, err := os.Stat(path)
	return err == nil
}

// NeedsSetup returns true if the task requires first-run setup.
func NeedsSetup(task Task) bool {
	return !PreferencesExist(task) && len(task.SetupQuestions()) > 0
}

// NeedsSetupIn returns true if the task requires first-run setup in a specific directory.
func NeedsSetupIn(task Task, dir string) bool {
	return !PreferencesExistIn(task, dir) && len(task.SetupQuestions()) > 0
}

// SetupResult contains the answers from a setup session.
type SetupResult struct {
	Answers map[string]string
}

// RunSetup executes the interactive setup flow for a task.
// It prompts the user for each setup question and returns the collected answers.
func RunSetup(task Task) (*SetupResult, error) {
	return RunSetupWithReader(task, os.Stdin)
}

// RunSetupWithReader executes the setup flow with a custom reader (for testing).
func RunSetupWithReader(task Task, reader *os.File) (*SetupResult, error) {
	questions := task.SetupQuestions()
	if len(questions) == 0 {
		return &SetupResult{Answers: make(map[string]string)}, nil
	}

	fmt.Printf("\nI don't have your %s preferences yet. Let me ask a few questions\n", task.Name())
	fmt.Println("to set things up. This only takes a minute.")
	fmt.Println()

	scanner := bufio.NewScanner(reader)
	answers := make(map[string]string)

	for _, q := range questions {
		answer, err := askQuestion(scanner, q)
		if err != nil {
			return nil, err
		}
		answers[q.Key] = answer
	}

	return &SetupResult{Answers: answers}, nil
}

// askQuestion prompts the user with a single question and returns the answer.
func askQuestion(scanner *bufio.Scanner, q SetupQuestion) (string, error) {
	switch q.Type {
	case QuestionConfirm:
		return askConfirm(scanner, q)
	case QuestionChoice:
		return askChoice(scanner, q)
	case QuestionMulti:
		return askMulti(scanner, q)
	default:
		return askText(scanner, q)
	}
}

// askText prompts for free text input.
func askText(scanner *bufio.Scanner, q SetupQuestion) (string, error) {
	if q.Default != "" {
		fmt.Printf("%s [%s]\n> ", q.Question, q.Default)
	} else {
		fmt.Printf("%s\n> ", q.Question)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return q.Default, nil
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return q.Default, nil
	}
	return answer, nil
}

// askConfirm prompts for yes/no input.
func askConfirm(scanner *bufio.Scanner, q SetupQuestion) (string, error) {
	defaultYes := strings.ToLower(q.Default) == "yes" || strings.ToLower(q.Default) == "y" || q.Default == ""

	var prompt string
	if defaultYes {
		prompt = fmt.Sprintf("%s [Y/n]\n> ", q.Question)
	} else {
		prompt = fmt.Sprintf("%s [y/N]\n> ", q.Question)
	}
	fmt.Print(prompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		if defaultYes {
			return "yes", nil
		}
		return "no", nil
	}

	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "" {
		if defaultYes {
			return "yes", nil
		}
		return "no", nil
	}

	if answer == "y" || answer == "yes" {
		return "yes", nil
	}
	return "no", nil
}

// askChoice prompts for single selection from options.
func askChoice(scanner *bufio.Scanner, q SetupQuestion) (string, error) {
	fmt.Println(q.Question)
	for i, opt := range q.Options {
		marker := "  "
		if opt == q.Default {
			marker = "* "
		}
		fmt.Printf("%s%d. %s\n", marker, i+1, opt)
	}
	fmt.Print("> ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return q.Default, nil
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return q.Default, nil
	}

	// Try parsing as a number
	if idx, err := strconv.Atoi(answer); err == nil {
		if idx >= 1 && idx <= len(q.Options) {
			return q.Options[idx-1], nil
		}
	}

	// Check if it matches an option directly
	for _, opt := range q.Options {
		if strings.EqualFold(answer, opt) {
			return opt, nil
		}
	}

	// Default to the input
	return answer, nil
}

// askMulti prompts for multiple selection from options.
func askMulti(scanner *bufio.Scanner, q SetupQuestion) (string, error) {
	fmt.Println(q.Question)
	fmt.Println("(Enter numbers separated by commas, or 'all')")
	for i, opt := range q.Options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}
	fmt.Print("> ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return q.Default, nil
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return q.Default, nil
	}

	if strings.ToLower(answer) == "all" {
		return strings.Join(q.Options, ","), nil
	}

	// Parse comma-separated numbers
	parts := strings.Split(answer, ",")
	selected := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if idx, err := strconv.Atoi(p); err == nil {
			if idx >= 1 && idx <= len(q.Options) {
				selected = append(selected, q.Options[idx-1])
			}
		}
	}

	if len(selected) == 0 {
		return q.Default, nil
	}
	return strings.Join(selected, ","), nil
}

// SavePreferences saves the setup results to a preferences file.
func SavePreferences(task Task, result *SetupResult) error {
	return SavePreferencesTo(task, result, DefaultPreferencesDir)
}

// SavePreferencesTo saves the setup results to a preferences file in a specific directory.
func SavePreferencesTo(task Task, result *SetupResult, dir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	path := filepath.Join(dir, task.PreferencesKey()+".md")

	// Group answers by section
	sections := make(map[string][]SetupQuestion)
	for _, q := range task.SetupQuestions() {
		section := q.Section
		if section == "" {
			section = "Settings"
		}
		sections[section] = append(sections[section], q)
	}

	// Build markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s Preferences\n\n", strings.Title(task.Name())))

	// Get section order (preserve order from questions)
	var sectionOrder []string
	seen := make(map[string]bool)
	for _, q := range task.SetupQuestions() {
		section := q.Section
		if section == "" {
			section = "Settings"
		}
		if !seen[section] {
			seen[section] = true
			sectionOrder = append(sectionOrder, section)
		}
	}

	for _, section := range sectionOrder {
		questions := sections[section]
		sb.WriteString(fmt.Sprintf("## %s\n\n", section))

		for _, q := range questions {
			value := result.Answers[q.Key]
			if value == "" {
				value = q.Default
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", q.Key, value))
		}
		sb.WriteString("\n")
	}

	// Write file
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write preferences file: %w", err)
	}

	fmt.Printf("\nI've saved your preferences to the Library. You can update them anytime\n")
	fmt.Printf("by running `hal9000 %s setup`.\n\n", task.Name())

	return nil
}
