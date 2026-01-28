// Package tasks provides the task framework for HAL 9000.
//
// Tasks are the CLI implementation of routines. Each task follows a consistent
// pattern so new tasks can be added easily using the same skeleton.
package tasks

import (
	"context"
)

// Task defines the interface all HAL tasks must implement.
type Task interface {
	// Name returns the task identifier (e.g., "agenda")
	Name() string

	// Description returns human-readable description
	Description() string

	// PreferencesKey returns the preferences file name (without .md extension)
	PreferencesKey() string

	// SetupQuestions returns questions for first-run setup
	SetupQuestions() []SetupQuestion

	// Run executes the task with given options
	Run(ctx context.Context, opts RunOptions) (*Result, error)
}

// SetupQuestion defines a question to ask during first-run setup.
type SetupQuestion struct {
	// Key is the preference key to set
	Key string

	// Question is the question to ask the user
	Question string

	// Default is the default value if user presses enter
	Default string

	// Options, if non-empty, makes this a multiple choice question
	Options []string

	// Section is the section in preferences file where this value goes
	Section string

	// Type specifies the question type (text, choice, multi, confirm, time)
	Type QuestionType
}

// QuestionType indicates the type of setup question.
type QuestionType string

const (
	// QuestionText is a free text input
	QuestionText QuestionType = "text"

	// QuestionChoice is a single selection from options
	QuestionChoice QuestionType = "choice"

	// QuestionMulti is multiple selection from options
	QuestionMulti QuestionType = "multi"

	// QuestionConfirm is a yes/no question
	QuestionConfirm QuestionType = "confirm"

	// QuestionTime is a time input
	QuestionTime QuestionType = "time"
)

// RunOptions contains options passed to task execution.
type RunOptions struct {
	// DryRun shows what would be done without doing it
	DryRun bool

	// Output overrides the default output location
	Output string

	// Format specifies output format (markdown, json, text)
	Format string

	// Args contains any additional arguments
	Args []string

	// Overrides contains one-time preference overrides
	Overrides map[string]string
}

// Result contains the outcome of a task execution.
type Result struct {
	// Success indicates whether the task completed successfully
	Success bool

	// Output is the generated output content
	Output string

	// OutputPath is where the output was written (if applicable)
	OutputPath string

	// Message is a human-readable summary of what happened
	Message string

	// Metadata contains any additional information about the execution
	Metadata map[string]interface{}
}
