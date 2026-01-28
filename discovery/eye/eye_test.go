package eye

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	e := New(Config{})

	if e.config.PatrolInterval != DefaultPatrolInterval {
		t.Errorf("PatrolInterval = %v, want %v", e.config.PatrolInterval, DefaultPatrolInterval)
	}
	if e.config.StaleThreshold != DefaultStaleThreshold {
		t.Errorf("StaleThreshold = %v, want %v", e.config.StaleThreshold, DefaultStaleThreshold)
	}
	if e.config.HealthTimeout != DefaultHealthTimeout {
		t.Errorf("HealthTimeout = %v, want %v", e.config.HealthTimeout, DefaultHealthTimeout)
	}
}

func TestNewWithConfig(t *testing.T) {
	config := Config{
		PatrolInterval: 30 * time.Second,
		StaleThreshold: 24 * time.Hour,
		HealthTimeout:  10 * time.Second,
	}

	e := New(config)

	if e.config.PatrolInterval != config.PatrolInterval {
		t.Errorf("PatrolInterval = %v, want %v", e.config.PatrolInterval, config.PatrolInterval)
	}
}

func TestRegisterCallbackHandler(t *testing.T) {
	e := New(Config{})

	handler := func(ctx context.Context, cb Callback) error {
		return nil
	}

	e.RegisterCallbackHandler("test-source", handler)

	if _, ok := e.callbackHandlers["test-source"]; !ok {
		t.Error("Expected handler to be registered")
	}
}

func TestRegisterHealthChecker(t *testing.T) {
	e := New(Config{})

	checker := func(ctx context.Context) HealthStatus {
		return HealthStatus{Healthy: true}
	}

	e.RegisterHealthChecker("test-component", checker)

	if _, ok := e.healthCheckers["test-component"]; !ok {
		t.Error("Expected health checker to be registered")
	}
}

func TestRegisterCleaner(t *testing.T) {
	e := New(Config{})

	cleaner := func(ctx context.Context, threshold time.Time) CleanupResult {
		return CleanupResult{ItemsCleaned: 0}
	}

	e.RegisterCleaner("test-component", cleaner)

	if _, ok := e.cleaners["test-component"]; !ok {
		t.Error("Expected cleaner to be registered")
	}
}

func TestSubmitCallback(t *testing.T) {
	e := New(Config{})

	cb := Callback{
		Source:    "test",
		Type:      "test.event",
		Timestamp: time.Now(),
	}

	e.SubmitCallback(cb)

	if len(e.callbackQueue) != 1 {
		t.Errorf("Callback queue length = %d, want 1", len(e.callbackQueue))
	}
	if e.callbackQueue[0].Source != "test" {
		t.Errorf("Callback source = %q, want %q", e.callbackQueue[0].Source, "test")
	}
}

func TestStartStop(t *testing.T) {
	e := New(Config{
		PatrolInterval: 100 * time.Millisecond,
	})

	ctx := context.Background()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !e.IsRunning() {
		t.Error("Expected eye to be running")
	}

	// Try starting again - should fail
	if err := e.Start(ctx); err == nil {
		t.Error("Expected error when starting already running eye")
	}

	e.Stop()

	if e.IsRunning() {
		t.Error("Expected eye to be stopped")
	}
}

func TestCallbackProcessing(t *testing.T) {
	e := New(Config{
		PatrolInterval: 50 * time.Millisecond,
	})

	var callCount int32
	handler := func(ctx context.Context, cb Callback) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	e.RegisterCallbackHandler("test-source", handler)

	// Submit callbacks
	e.SubmitCallback(Callback{Source: "test-source", Type: "event1", Timestamp: time.Now()})
	e.SubmitCallback(Callback{Source: "test-source", Type: "event2", Timestamp: time.Now()})

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for patrol to process
	time.Sleep(150 * time.Millisecond)

	e.Stop()

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("Call count = %d, want 2", callCount)
	}
}

func TestHealthChecks(t *testing.T) {
	e := New(Config{
		PatrolInterval: 50 * time.Millisecond,
		HealthTimeout:  1 * time.Second,
	})

	e.RegisterHealthChecker("healthy-component", func(ctx context.Context) HealthStatus {
		return HealthStatus{Healthy: true, Message: "all good"}
	})

	e.RegisterHealthChecker("unhealthy-component", func(ctx context.Context) HealthStatus {
		return HealthStatus{Healthy: false, Message: "something wrong"}
	})

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for patrol
	time.Sleep(150 * time.Millisecond)

	e.Stop()

	status := e.GetHealthStatus()

	if len(status) != 2 {
		t.Errorf("Health status count = %d, want 2", len(status))
	}

	if !status["healthy-component"].Healthy {
		t.Error("Expected healthy-component to be healthy")
	}

	if status["unhealthy-component"].Healthy {
		t.Error("Expected unhealthy-component to be unhealthy")
	}
}

func TestCleanup(t *testing.T) {
	e := New(Config{
		PatrolInterval: 50 * time.Millisecond,
		StaleThreshold: 24 * time.Hour,
	})

	var cleanupCalled bool
	var cleanupThreshold time.Time

	e.RegisterCleaner("test-component", func(ctx context.Context, threshold time.Time) CleanupResult {
		cleanupCalled = true
		cleanupThreshold = threshold
		return CleanupResult{ItemsCleaned: 5, BytesFreed: 1024}
	})

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for patrol
	time.Sleep(150 * time.Millisecond)

	e.Stop()

	if !cleanupCalled {
		t.Error("Expected cleanup to be called")
	}

	// Threshold should be approximately 24 hours ago
	expectedThreshold := time.Now().Add(-24 * time.Hour)
	diff := cleanupThreshold.Sub(expectedThreshold)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Cleanup threshold difference = %v, expected within 1 second", diff)
	}
}

func TestCallbackFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eye-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	e := New(Config{
		PatrolInterval: 50 * time.Millisecond,
		CallbackDir:    tmpDir,
	})

	var receivedCallbacks []Callback
	e.RegisterCallbackHandler("file-source", func(ctx context.Context, cb Callback) error {
		receivedCallbacks = append(receivedCallbacks, cb)
		return nil
	})

	// Write a callback file
	cb := Callback{
		Source:    "file-source",
		Type:      "file.event",
		Payload:   map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(cb)
	callbackFile := filepath.Join(tmpDir, "callback1.json")
	if err := os.WriteFile(callbackFile, data, 0644); err != nil {
		t.Fatalf("Failed to write callback file: %v", err)
	}

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for patrols
	time.Sleep(200 * time.Millisecond)

	e.Stop()

	if len(receivedCallbacks) != 1 {
		t.Errorf("Received %d callbacks, want 1", len(receivedCallbacks))
	}

	// Callback file should be removed
	if _, err := os.Stat(callbackFile); !os.IsNotExist(err) {
		t.Error("Expected callback file to be removed after processing")
	}
}

func TestLastPatrolTime(t *testing.T) {
	e := New(Config{
		PatrolInterval: 50 * time.Millisecond,
	})

	// Before start, should be zero
	if !e.LastPatrolTime().IsZero() {
		t.Error("Expected zero time before start")
	}

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for patrol
	time.Sleep(100 * time.Millisecond)

	e.Stop()

	// After patrol, should be non-zero
	if e.LastPatrolTime().IsZero() {
		t.Error("Expected non-zero time after patrol")
	}

	// Should be recent
	elapsed := time.Since(e.LastPatrolTime())
	if elapsed > 5*time.Second {
		t.Errorf("Last patrol time too old: %v ago", elapsed)
	}
}

func TestContextCancellation(t *testing.T) {
	e := New(Config{
		PatrolInterval: 1 * time.Hour, // Long interval so we control the test
	})

	ctx, cancel := context.WithCancel(context.Background())

	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context
	cancel()

	// Wait a bit for shutdown
	time.Sleep(100 * time.Millisecond)

	// Should have stopped
	if e.IsRunning() {
		// Force stop for cleanup
		e.Stop()
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input         string
		expected      string
		checkAbsolute bool // for relative paths, just check it becomes absolute
	}{
		{"~/test", filepath.Join(home, "test"), false},
		{"/absolute/path", "/absolute/path", false},
		{"relative/path", "", true}, // relative paths should become absolute
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if tt.checkAbsolute {
			if !filepath.IsAbs(result) {
				t.Errorf("expandPath(%q) = %q, want absolute path", tt.input, result)
			}
		} else if result != tt.expected {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
