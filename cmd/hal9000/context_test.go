package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandContextPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"~/.config", filepath.Join(home, ".config")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandContextPath(tt.input)
			if got != tt.want {
				t.Errorf("expandContextPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadServicesConfig(t *testing.T) {
	// Test that loadServicesConfig returns at least the default services
	// when config file doesn't exist
	services := loadServicesConfig()

	// Should return something even if file doesn't exist
	if services == nil {
		t.Error("loadServicesConfig should not return nil")
	}
}

func TestCheckIntegrations(t *testing.T) {
	integrations := checkIntegrations()

	if len(integrations) == 0 {
		t.Error("checkIntegrations should return at least one integration")
	}

	// Check that expected integrations are present
	integrationNames := make(map[string]bool)
	for _, i := range integrations {
		integrationNames[i.Name] = true
	}

	expectedIntegrations := []string{"calendar", "jira", "slack"}
	for _, expected := range expectedIntegrations {
		if !integrationNames[expected] {
			t.Errorf("expected integration %q not found", expected)
		}
	}
}

func TestGetSchedulerStatus(t *testing.T) {
	// This test just verifies the function doesn't panic
	status := getSchedulerStatus()

	// Status should be either "running" or "stopped"
	if status != "running" && status != "stopped" {
		t.Errorf("getSchedulerStatus returned unexpected status: %q", status)
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Test with current process (should be alive)
	if !isProcessAlive(os.Getpid()) {
		t.Error("isProcessAlive should return true for current process")
	}

	// Test with invalid PID (should not be alive)
	// Use a very high PID that's unlikely to exist
	if isProcessAlive(999999999) {
		t.Error("isProcessAlive should return false for non-existent process")
	}
}

func TestReadPIDFile(t *testing.T) {
	// Create temp file with a PID
	tmpDir, err := os.MkdirTemp("", "context-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pidPath := filepath.Join(tmpDir, "test.pid")

	// Test reading non-existent file
	_, err = readPIDFile(pidPath)
	if err == nil {
		t.Error("readPIDFile should return error for non-existent file")
	}

	// Write a valid PID file
	if err := os.WriteFile(pidPath, []byte("12345\n"), 0644); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := readPIDFile(pidPath)
	if err != nil {
		t.Errorf("readPIDFile returned error: %v", err)
	}
	if pid != 12345 {
		t.Errorf("readPIDFile returned wrong PID: got %d, want 12345", pid)
	}
}

func TestGetServiceStatus(t *testing.T) {
	// Test with a disabled service
	svc := ServiceConfig{
		Name:        "test-service",
		Description: "Test service",
		Enabled:     false,
	}

	status := getServiceStatus(svc)

	// Should return "stopped" for a service without a PID file
	if status != "stopped" {
		t.Errorf("getServiceStatus returned %q, expected 'stopped'", status)
	}
}
