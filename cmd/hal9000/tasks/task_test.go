package tasks

import (
	"context"
	"testing"
)

// mockTask implements Task for testing.
type mockTask struct {
	name           string
	description    string
	preferencesKey string
	setupQuestions []SetupQuestion
	runFunc        func(ctx context.Context, opts RunOptions) (*Result, error)
}

func (m *mockTask) Name() string        { return m.name }
func (m *mockTask) Description() string { return m.description }
func (m *mockTask) PreferencesKey() string {
	if m.preferencesKey != "" {
		return m.preferencesKey
	}
	return m.name
}
func (m *mockTask) SetupQuestions() []SetupQuestion { return m.setupQuestions }
func (m *mockTask) Run(ctx context.Context, opts RunOptions) (*Result, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, opts)
	}
	return &Result{Success: true, Message: "Task completed"}, nil
}

func TestTaskInterface(t *testing.T) {
	task := &mockTask{
		name:        "test",
		description: "A test task",
		setupQuestions: []SetupQuestion{
			{Key: "key1", Question: "Question 1?", Default: "default1", Type: QuestionText},
			{Key: "key2", Question: "Question 2?", Options: []string{"a", "b"}, Type: QuestionChoice},
		},
	}

	if task.Name() != "test" {
		t.Errorf("Name() = %q, want %q", task.Name(), "test")
	}

	if task.Description() != "A test task" {
		t.Errorf("Description() = %q, want %q", task.Description(), "A test task")
	}

	if task.PreferencesKey() != "test" {
		t.Errorf("PreferencesKey() = %q, want %q", task.PreferencesKey(), "test")
	}

	questions := task.SetupQuestions()
	if len(questions) != 2 {
		t.Errorf("SetupQuestions() returned %d questions, want 2", len(questions))
	}
}

func TestRunOptions(t *testing.T) {
	opts := RunOptions{
		DryRun: true,
		Output: "/tmp/output.md",
		Format: "json",
		Args:   []string{"arg1", "arg2"},
		Overrides: map[string]string{
			"key": "value",
		},
	}

	if !opts.DryRun {
		t.Error("DryRun should be true")
	}
	if opts.Output != "/tmp/output.md" {
		t.Errorf("Output = %q, want %q", opts.Output, "/tmp/output.md")
	}
	if opts.Format != "json" {
		t.Errorf("Format = %q, want %q", opts.Format, "json")
	}
	if len(opts.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(opts.Args))
	}
}

func TestResult(t *testing.T) {
	result := &Result{
		Success:    true,
		Output:     "# Agenda\n\nToday's agenda...",
		OutputPath: "/library/agenda/2026-01-28.md",
		Message:    "Created daily agenda",
		Metadata: map[string]interface{}{
			"items_count": 5,
		},
	}

	if !result.Success {
		t.Error("Success should be true")
	}
	if result.OutputPath != "/library/agenda/2026-01-28.md" {
		t.Errorf("OutputPath = %q, want %q", result.OutputPath, "/library/agenda/2026-01-28.md")
	}
}

func TestQuestionTypes(t *testing.T) {
	tests := []struct {
		qtype QuestionType
		want  string
	}{
		{QuestionText, "text"},
		{QuestionChoice, "choice"},
		{QuestionMulti, "multi"},
		{QuestionConfirm, "confirm"},
		{QuestionTime, "time"},
	}

	for _, tt := range tests {
		if string(tt.qtype) != tt.want {
			t.Errorf("QuestionType = %q, want %q", string(tt.qtype), tt.want)
		}
	}
}
