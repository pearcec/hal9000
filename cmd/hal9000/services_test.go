package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pearcec/hal9000/internal/config"
	"gopkg.in/yaml.v3"
)

func TestGetDefaultServicesConfig(t *testing.T) {
	config := getDefaultServicesConfig()

	if len(config.Services) != 4 {
		t.Errorf("expected 4 services, got %d", len(config.Services))
	}

	// Check service names
	expectedNames := []string{"scheduler", "floyd-calendar", "floyd-jira", "floyd-slack"}
	for i, name := range expectedNames {
		if config.Services[i].Name != name {
			t.Errorf("expected service %d to be %s, got %s", i, name, config.Services[i].Name)
		}
	}

	// Scheduler should be enabled by default
	if !config.Services[0].Enabled {
		t.Error("scheduler should be enabled by default")
	}

	// Floyd services should be disabled by default
	for i := 1; i < 4; i++ {
		if config.Services[i].Enabled {
			t.Errorf("%s should be disabled by default", config.Services[i].Name)
		}
	}
}

func TestFindService(t *testing.T) {
	config := getDefaultServicesConfig()

	// Find existing service
	svc := findService(config, "scheduler")
	if svc == nil {
		t.Fatal("expected to find scheduler service")
	}
	if svc.Name != "scheduler" {
		t.Errorf("expected name scheduler, got %s", svc.Name)
	}

	// Find non-existing service
	svc = findService(config, "nonexistent")
	if svc != nil {
		t.Error("expected nil for nonexistent service")
	}
}

func TestServicesConfigYAML(t *testing.T) {
	config := &ServicesConfig{
		Services: []ServiceConfig{
			{
				Name:        "test-service",
				Command:     "/usr/bin/test",
				Args:        []string{"--arg1", "--arg2"},
				Enabled:     true,
				Description: "Test service",
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// Unmarshal back
	var parsed ServicesConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(parsed.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(parsed.Services))
	}

	svc := parsed.Services[0]
	if svc.Name != "test-service" {
		t.Errorf("expected name test-service, got %s", svc.Name)
	}
	if svc.Command != "/usr/bin/test" {
		t.Errorf("expected command /usr/bin/test, got %s", svc.Command)
	}
	if len(svc.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(svc.Args))
	}
	if !svc.Enabled {
		t.Error("expected service to be enabled")
	}
}

func TestServicePIDPaths(t *testing.T) {
	// Test PID path generation
	pidPath := getServicePIDPath("scheduler")
	if !filepath.IsAbs(pidPath) {
		t.Errorf("expected absolute path, got %s", pidPath)
	}
	if filepath.Base(pidPath) != "scheduler.pid" {
		t.Errorf("expected scheduler.pid, got %s", filepath.Base(pidPath))
	}
}

func TestServiceLogPaths(t *testing.T) {
	// Test log path generation
	logPath := getServiceLogPath("scheduler")
	if !filepath.IsAbs(logPath) {
		t.Errorf("expected absolute path, got %s", logPath)
	}
	if filepath.Base(logPath) != "scheduler.log" {
		t.Errorf("expected scheduler.log, got %s", filepath.Base(logPath))
	}
	if filepath.Base(filepath.Dir(logPath)) != "logs" {
		t.Errorf("expected logs directory, got %s", filepath.Base(filepath.Dir(logPath)))
	}
}

func TestLoadServicesConfigDefault(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config when file doesn't exist
	config, err := loadServicesConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	if len(config.Services) != 4 {
		t.Errorf("expected 4 default services, got %d", len(config.Services))
	}
}

func TestLoadServicesConfigFromFile(t *testing.T) {
	// Create temp directory as project directory
	tmpDir := t.TempDir()

	// Set up executable directory for testing (services.yaml is project-relative)
	config.ResetForTesting()
	config.SetExecutableDirForTesting(tmpDir)
	defer config.ResetForTesting()

	// Write test config to project directory
	testConfig := `services:
  - name: custom-service
    command: /custom/cmd
    enabled: true
`
	configPath := filepath.Join(tmpDir, "services.yaml")
	if err := os.WriteFile(configPath, []byte(testConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	svcConfig, err := loadServicesConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(svcConfig.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcConfig.Services))
	}

	if svcConfig.Services[0].Name != "custom-service" {
		t.Errorf("expected custom-service, got %s", svcConfig.Services[0].Name)
	}
}

func TestSaveServicesConfig(t *testing.T) {
	// Create temp directory as project directory
	tmpDir := t.TempDir()

	// Set up executable directory for testing (services.yaml is project-relative)
	config.ResetForTesting()
	config.SetExecutableDirForTesting(tmpDir)
	defer config.ResetForTesting()

	svcConfig := &ServicesConfig{
		Services: []ServiceConfig{
			{
				Name:    "test",
				Command: "/test",
				Enabled: true,
			},
		},
	}

	if err := saveServicesConfig(svcConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists in project directory
	configPath := filepath.Join(tmpDir, "services.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Should contain header comment
	if len(data) < 100 {
		t.Error("config file seems too short, might be missing header")
	}
}

func TestServiceStatusStruct(t *testing.T) {
	status := ServiceStatus{
		Name:        "scheduler",
		Running:     true,
		PID:         12345,
		Uptime:      "2h 15m",
		Description: "HAL task scheduler",
		Enabled:     true,
	}

	if status.Name != "scheduler" {
		t.Errorf("expected scheduler, got %s", status.Name)
	}
	if !status.Running {
		t.Error("expected running to be true")
	}
	if status.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", status.PID)
	}
}
