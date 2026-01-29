package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDirIfNotExists(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	var created []string

	// Test creating a new directory
	testPath := filepath.Join("test", "subdir")
	err = createDirIfNotExists(testPath, &created)
	if err != nil {
		t.Errorf("createDirIfNotExists failed: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 created item, got %d", len(created))
	}

	// Verify directory exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Test idempotency - creating same directory again
	created = []string{}
	err = createDirIfNotExists(testPath, &created)
	if err != nil {
		t.Errorf("createDirIfNotExists failed on second call: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 created items on idempotent call, got %d", len(created))
	}
}

func TestCreateConfigIfNotExists(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var created []string

	// Test creating a new config file
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = createConfigIfNotExists(configPath, &created)
	if err != nil {
		t.Errorf("createConfigIfNotExists failed: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 created item, got %d", len(created))
	}

	// Verify file exists and has content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Config file is empty")
	}

	// Test idempotency - creating same file again
	created = []string{}
	err = createConfigIfNotExists(configPath, &created)
	if err != nil {
		t.Errorf("createConfigIfNotExists failed on second call: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 created items on idempotent call, got %d", len(created))
	}
}

func TestCreateServicesConfigWithSelection(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-services-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var created []string

	// Test creating a new services config file with default selection
	servicesPath := filepath.Join(tmpDir, "services.yaml")
	selection := ServiceSelection{
		Scheduler: true,
		Calendar:  false,
		Jira:      false,
		Slack:     false,
	}
	err = createServicesConfigWithSelection(servicesPath, selection, &created)
	if err != nil {
		t.Errorf("createServicesConfigWithSelection failed: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 created item, got %d", len(created))
	}

	// Verify file exists and has content
	content, err := os.ReadFile(servicesPath)
	if err != nil {
		t.Errorf("Failed to read services file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Services file is empty")
	}

	// Verify content includes expected sections
	contentStr := string(content)
	expectedSections := []string{
		"services:",
		"name: scheduler",
		"name: floyd-calendar",
		"name: floyd-jira",
		"name: floyd-slack",
	}

	for _, section := range expectedSections {
		if !contains(contentStr, section) {
			t.Errorf("Services config missing expected section: %s", section)
		}
	}

	// Test idempotency - creating same file again
	created = []string{}
	err = createServicesConfigWithSelection(servicesPath, selection, &created)
	if err != nil {
		t.Errorf("createServicesConfigWithSelection failed on second call: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 created items on idempotent call, got %d", len(created))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc12345678", "abc12***"},
		{"short", "*****"},
		{"abc", "***"},
		{"", ""},
		{"12345", "*****"},
		{"123456", "12345***"},
		{"xoxb-1234567890-abcdefghij", "xoxb-***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskSecret(tt.input)
			if result != tt.expected {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadJIRACredentials(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-jira-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test loading non-existent credentials
	_, err = loadJIRACredentials(tmpDir)
	if err == nil {
		t.Error("Expected error loading non-existent credentials")
	}

	// Create test credentials file
	credsContent := `url: https://test.atlassian.net
email: test@example.com
api_token: secret123
`
	credsPath := filepath.Join(tmpDir, "jira-credentials.yaml")
	if err := os.WriteFile(credsPath, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write test credentials: %v", err)
	}

	// Test loading existing credentials
	creds, err := loadJIRACredentials(tmpDir)
	if err != nil {
		t.Errorf("Failed to load credentials: %v", err)
	}
	if creds.URL != "https://test.atlassian.net" {
		t.Errorf("Expected URL 'https://test.atlassian.net', got %q", creds.URL)
	}
	if creds.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %q", creds.Email)
	}
	if creds.APIToken != "secret123" {
		t.Errorf("Expected api_token 'secret123', got %q", creds.APIToken)
	}
}

func TestLoadSlackCredentials(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-slack-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test loading non-existent credentials
	_, err = loadSlackCredentials(tmpDir)
	if err == nil {
		t.Error("Expected error loading non-existent credentials")
	}

	// Create test credentials file
	credsContent := `bot_token: xoxb-test-token
`
	credsPath := filepath.Join(tmpDir, "slack-credentials.yaml")
	if err := os.WriteFile(credsPath, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write test credentials: %v", err)
	}

	// Test loading existing credentials
	creds, err := loadSlackCredentials(tmpDir)
	if err != nil {
		t.Errorf("Failed to load credentials: %v", err)
	}
	if creds.BotToken != "xoxb-test-token" {
		t.Errorf("Expected bot_token 'xoxb-test-token', got %q", creds.BotToken)
	}
}

func TestLoadBambooHRCredentials(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-bamboohr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test loading non-existent credentials
	_, err = loadBambooHRCredentials(tmpDir)
	if err == nil {
		t.Error("Expected error loading non-existent credentials")
	}

	// Create test credentials file
	credsContent := `subdomain: mycompany
api_key: bamboo-api-key-12345
`
	credsPath := filepath.Join(tmpDir, "bamboohr-credentials.yaml")
	if err := os.WriteFile(credsPath, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write test credentials: %v", err)
	}

	// Test loading existing credentials
	creds, err := loadBambooHRCredentials(tmpDir)
	if err != nil {
		t.Errorf("Failed to load credentials: %v", err)
	}
	if creds.Subdomain != "mycompany" {
		t.Errorf("Expected subdomain 'mycompany', got %q", creds.Subdomain)
	}
	if creds.APIKey != "bamboo-api-key-12345" {
		t.Errorf("Expected api_key 'bamboo-api-key-12345', got %q", creds.APIKey)
	}
}

func TestInitCreatesAllLibraryDirs(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-init-full-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	expectedDirs := []string{
		"library/agenda",
		"library/preferences",
		"library/people-profiles",
		"library/collaborations",
		"library/url_library",
		"library/reminders",
		"library/hal-memory",
		"library/calendar",
		"library/schedules",
		"library/logs",
	}

	var created []string

	// Create all library directories
	for _, dir := range expectedDirs {
		err := createDirIfNotExists(dir, &created)
		if err != nil {
			t.Errorf("Failed to create %s: %v", dir, err)
		}
	}

	// Verify all directories exist
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}

	if len(created) != len(expectedDirs) {
		t.Errorf("Expected %d created dirs, got %d", len(expectedDirs), len(created))
	}
}
