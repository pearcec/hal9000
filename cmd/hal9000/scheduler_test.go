package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduleStruct(t *testing.T) {
	schedule := Schedule{
		Task:    "agenda",
		Cron:    "0 6 * * *",
		Enabled: true,
		Notify:  true,
	}

	if schedule.Task != "agenda" {
		t.Errorf("expected task 'agenda', got %s", schedule.Task)
	}
	if schedule.Cron != "0 6 * * *" {
		t.Errorf("expected cron '0 6 * * *', got %s", schedule.Cron)
	}
	if !schedule.Enabled {
		t.Error("expected schedule to be enabled")
	}
	if !schedule.Notify {
		t.Error("expected notify to be true")
	}
}

func TestSchedulerConfigJSON(t *testing.T) {
	config := SchedulerConfig{
		Schedules: []Schedule{
			{Task: "agenda", Cron: "0 6 * * *", Enabled: true, Notify: true},
			{Task: "weekly-review", Cron: "0 16 * * 5", Enabled: false, Notify: false},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var parsed SchedulerConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(parsed.Schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(parsed.Schedules))
	}

	if parsed.Schedules[0].Task != "agenda" {
		t.Errorf("expected first task 'agenda', got %s", parsed.Schedules[0].Task)
	}
	if parsed.Schedules[1].Enabled {
		t.Error("expected second schedule to be disabled")
	}
}

func TestFindSchedule(t *testing.T) {
	config := &SchedulerConfig{
		Schedules: []Schedule{
			{Task: "agenda", Cron: "0 6 * * *", Enabled: true},
			{Task: "weekly-review", Cron: "0 16 * * 5", Enabled: true},
		},
	}

	// Find existing task
	idx := findSchedule(config, "agenda")
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	idx = findSchedule(config, "weekly-review")
	if idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}

	// Find non-existent task
	idx = findSchedule(config, "nonexistent")
	if idx != -1 {
		t.Errorf("expected index -1, got %d", idx)
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Test with our own process (should be running)
	running := isProcessRunning(os.Getpid())
	if !running {
		t.Error("expected own process to be detected as running")
	}

	// Test with an obviously invalid PID (should not be running)
	// Note: PID 1 is init/systemd on Unix, which is always running,
	// so we use a very high PID that's unlikely to exist
	running = isProcessRunning(999999999)
	if running {
		t.Error("expected invalid PID to be detected as not running")
	}
}

func TestExecutionResultJSON(t *testing.T) {
	result := ExecutionResult{
		Task:      "test-task",
		Timestamp: time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC),
		Success:   true,
		Duration:  "1.5s",
		Message:   "Task completed successfully",
		Output:    "Sample output",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var parsed ExecutionResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if parsed.Task != "test-task" {
		t.Errorf("expected task 'test-task', got %s", parsed.Task)
	}
	if !parsed.Success {
		t.Error("expected success to be true")
	}
	if parsed.Duration != "1.5s" {
		t.Errorf("expected duration '1.5s', got %s", parsed.Duration)
	}
}

func TestStoreExecutionResult(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalLibPath := schedulerLibPath
	schedulerLibPath = tmpDir
	defer func() { schedulerLibPath = originalLibPath }()

	// Store a result
	err := storeExecutionResult("test-task", true, "Test message", "Test output", time.Second)
	if err != nil {
		t.Fatalf("failed to store result: %v", err)
	}

	// Verify the file was created
	resultsFile := filepath.Join(tmpDir, "scheduler-results", "test-task.jsonl")
	if _, err := os.Stat(resultsFile); os.IsNotExist(err) {
		t.Fatal("results file was not created")
	}

	// Read and parse the result
	data, err := os.ReadFile(resultsFile)
	if err != nil {
		t.Fatalf("failed to read results file: %v", err)
	}

	var result ExecutionResult
	if err := json.Unmarshal(data[:len(data)-1], &result); err != nil { // -1 to remove trailing newline
		t.Fatalf("failed to parse result: %v", err)
	}

	if result.Task != "test-task" {
		t.Errorf("expected task 'test-task', got %s", result.Task)
	}
	if !result.Success {
		t.Error("expected success to be true")
	}
	if result.Message != "Test message" {
		t.Errorf("expected message 'Test message', got %s", result.Message)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	// Save and restore original library path
	originalLibPath := schedulerLibPath
	schedulerLibPath = t.TempDir()
	defer func() { schedulerLibPath = originalLibPath }()

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}
	if config == nil {
		t.Fatal("expected config to be non-nil")
	}
	if len(config.Schedules) != 0 {
		t.Errorf("expected empty schedules, got %d", len(config.Schedules))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Save and restore original library path
	originalLibPath := schedulerLibPath
	schedulerLibPath = t.TempDir()
	defer func() { schedulerLibPath = originalLibPath }()

	// Save config
	config := &SchedulerConfig{
		Schedules: []Schedule{
			{Task: "agenda", Cron: "0 6 * * *", Enabled: true, Notify: true},
		},
	}

	if err := saveConfig(config); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load it back
	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(loaded.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(loaded.Schedules))
	}

	if loaded.Schedules[0].Task != "agenda" {
		t.Errorf("expected task 'agenda', got %s", loaded.Schedules[0].Task)
	}
	if loaded.Schedules[0].Cron != "0 6 * * *" {
		t.Errorf("expected cron '0 6 * * *', got %s", loaded.Schedules[0].Cron)
	}
	if !loaded.Schedules[0].Enabled {
		t.Error("expected schedule to be enabled")
	}
	if !loaded.Schedules[0].Notify {
		t.Error("expected notify to be true")
	}
}
