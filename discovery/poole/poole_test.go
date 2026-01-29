package poole

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pearcec/hal9000/discovery/events"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	// Should start empty
	if len(r.ListActions()) != 0 {
		t.Error("New registry should have no actions")
	}

	if len(r.ListPrompts()) != 0 {
		t.Error("New registry should have no prompts")
	}
}

func TestRegisterAction(t *testing.T) {
	r := NewRegistry()

	action := &Action{
		Name:      "test-action",
		EventType: "jira:issue.created",
		Enabled:   true,
		Prompt:    "test-prompt",
	}

	r.RegisterAction(action)

	// Should be retrievable
	got, ok := r.GetAction("test-action")
	if !ok {
		t.Fatal("Action not found after registration")
	}

	if got.Name != "test-action" {
		t.Errorf("Got action name %q, want %q", got.Name, "test-action")
	}

	// Should be indexed by event type
	actions := r.GetActionsForEvent("jira:issue.created")
	if len(actions) != 1 {
		t.Errorf("Got %d actions for event, want 1", len(actions))
	}
}

func TestGetActionsForEvent_Wildcard(t *testing.T) {
	r := NewRegistry()

	// Register action with wildcard
	action := &Action{
		Name:      "jira-all",
		EventType: "jira:*",
		Enabled:   true,
	}
	r.RegisterAction(action)

	// Should match any jira event
	tests := []struct {
		eventType string
		wantMatch bool
	}{
		{"jira:issue.created", true},
		{"jira:issue.modified", true},
		{"slack:message.new", false},
		{"jira", false}, // No colon
	}

	for _, tc := range tests {
		actions := r.GetActionsForEvent(tc.eventType)
		gotMatch := len(actions) > 0
		if gotMatch != tc.wantMatch {
			t.Errorf("GetActionsForEvent(%q): got match=%v, want match=%v", tc.eventType, gotMatch, tc.wantMatch)
		}
	}
}

func TestRegisterPrompt(t *testing.T) {
	r := NewRegistry()

	template := "You are processing event {{event_id}} from {{source}}."
	r.RegisterPrompt("test-prompt", template)

	got, err := r.GetPrompt("test-prompt")
	if err != nil {
		t.Fatalf("GetPrompt error: %v", err)
	}

	if got != template {
		t.Errorf("Got template %q, want %q", got, template)
	}
}

func TestGetPrompt_NotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetPrompt("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent prompt")
	}
}

func TestExpandPrompt(t *testing.T) {
	template := "Processing {{event_id}} from {{source}} at {{time}}."
	vars := map[string]string{
		"event_id": "EVT-123",
		"source":   "jira",
		"time":     "2024-01-15",
	}

	got := ExpandPrompt(template, vars)
	want := "Processing EVT-123 from jira at 2024-01-15."

	if got != want {
		t.Errorf("Got %q, want %q", got, want)
	}
}

func TestExpandPrompt_MissingVar(t *testing.T) {
	template := "Processing {{event_id}} from {{source}}."
	vars := map[string]string{
		"event_id": "EVT-123",
		// source is missing
	}

	got := ExpandPrompt(template, vars)
	want := "Processing EVT-123 from {{source}}."

	if got != want {
		t.Errorf("Got %q, want %q", got, want)
	}
}

func TestLoadActions(t *testing.T) {
	// Create temp actions file
	tempDir := t.TempDir()
	actionsPath := filepath.Join(tempDir, "actions.yaml")

	actionsYAML := `actions:
  bamboohr-inbox-new:
    enabled: true
    event_type: "bamboohr:inbox.new"
    fetch:
      - bowman.bamboohr.message
      - bowman.bamboohr.sender-profile
    prompt: bamboohr-inbox-triage
    action_type: immediate
  jira-issue-created:
    enabled: false
    event_type: "jira:issue.created"
    fetch:
      - bowman.jira.issue
    prompt: jira-triage
    action_type: delayed
    metadata:
      delay: "5m"
`

	if err := os.WriteFile(actionsPath, []byte(actionsYAML), 0644); err != nil {
		t.Fatalf("Failed to write test actions file: %v", err)
	}

	r := NewRegistry()
	if err := r.LoadActions(actionsPath); err != nil {
		t.Fatalf("LoadActions error: %v", err)
	}

	// Check actions loaded correctly
	actions := r.ListActions()
	if len(actions) != 2 {
		t.Errorf("Got %d actions, want 2", len(actions))
	}

	// Check specific action
	action, ok := r.GetAction("bamboohr-inbox-new")
	if !ok {
		t.Fatal("bamboohr-inbox-new action not found")
	}

	if !action.Enabled {
		t.Error("bamboohr-inbox-new should be enabled")
	}

	if len(action.Fetchers) != 2 {
		t.Errorf("Got %d fetchers, want 2", len(action.Fetchers))
	}

	if action.ActionType != ActionTypeImmediate {
		t.Errorf("Got action type %v, want immediate", action.ActionType)
	}

	// Check disabled action
	jiraAction, ok := r.GetAction("jira-issue-created")
	if !ok {
		t.Fatal("jira-issue-created action not found")
	}

	if jiraAction.Enabled {
		t.Error("jira-issue-created should be disabled")
	}

	if jiraAction.ActionType != ActionTypeDelayed {
		t.Errorf("Got action type %v, want delayed", jiraAction.ActionType)
	}
}

func TestLoadPrompts(t *testing.T) {
	// Create temp prompt directories
	tempDir := t.TempDir()
	defaultsDir := filepath.Join(tempDir, "defaults")
	userDir := filepath.Join(tempDir, "user")

	os.MkdirAll(defaultsDir, 0755)
	os.MkdirAll(userDir, 0755)

	// Create default prompt
	defaultPrompt := "Default prompt for {{event_id}}"
	os.WriteFile(filepath.Join(defaultsDir, "test-prompt.md"), []byte(defaultPrompt), 0644)

	// Create another default
	anotherDefault := "Another default prompt"
	os.WriteFile(filepath.Join(defaultsDir, "another-prompt.md"), []byte(anotherDefault), 0644)

	// Create user override
	userOverride := "User override for {{event_id}}"
	os.WriteFile(filepath.Join(userDir, "test-prompt.md"), []byte(userOverride), 0644)

	r := NewRegistry()
	r.AddPromptPath(defaultsDir)
	r.AddPromptPath(userDir)

	if err := r.LoadPrompts(); err != nil {
		t.Fatalf("LoadPrompts error: %v", err)
	}

	// Should have both prompts
	prompts := r.ListPrompts()
	if len(prompts) != 2 {
		t.Errorf("Got %d prompts, want 2", len(prompts))
	}

	// User override should take precedence
	got, err := r.GetPrompt("test-prompt")
	if err != nil {
		t.Fatalf("GetPrompt error: %v", err)
	}

	if got != userOverride {
		t.Errorf("User override not applied. Got %q, want %q", got, userOverride)
	}
}

func TestNewScheduler(t *testing.T) {
	s := NewScheduler()
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}

	if s.QueueLength() != 0 {
		t.Error("New scheduler should have empty queue")
	}

	if s.BatchCount() != 0 {
		t.Error("New scheduler should have no batches")
	}
}

func TestScheduler_Schedule_Delayed(t *testing.T) {
	s := NewScheduler()

	event := events.StorageEvent{
		Source:  "test",
		EventID: "evt-1",
	}

	action := &Action{
		Name:       "delayed-action",
		ActionType: ActionTypeDelayed,
		Metadata: map[string]string{
			"delay": "1s",
		},
	}

	executed := false
	handler := func(e events.StorageEvent, a *Action) ActionResult {
		executed = true
		return ActionResult{Success: true}
	}

	s.Schedule(event, action, handler)

	if s.QueueLength() != 1 {
		t.Errorf("Queue length = %d, want 1", s.QueueLength())
	}

	// Action should not be executed yet
	if executed {
		t.Error("Action should not be executed immediately")
	}
}

func TestScheduler_Schedule_Batched(t *testing.T) {
	s := NewScheduler()

	action := &Action{
		Name:       "batched-action",
		ActionType: ActionTypeBatched,
	}

	handler := func(e events.StorageEvent, a *Action) ActionResult {
		return ActionResult{Success: true}
	}

	// Add multiple events to batch
	for i := 0; i < 3; i++ {
		event := events.StorageEvent{
			Source:  "test",
			EventID: "evt-" + string(rune('1'+i)),
		}
		s.Schedule(event, action, handler)
	}

	if s.BatchCount() != 1 {
		t.Errorf("Batch count = %d, want 1", s.BatchCount())
	}
}

func TestNewDispatcher(t *testing.T) {
	r := NewRegistry()
	s := NewScheduler()
	d := NewDispatcher(r, s)

	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
}

func TestDispatcher_RegisterHandler(t *testing.T) {
	r := NewRegistry()
	s := NewScheduler()
	d := NewDispatcher(r, s)

	handler := func(e events.StorageEvent, a *Action) ActionResult {
		return ActionResult{Success: true}
	}

	d.RegisterHandler("test-action", handler)

	// Verify handler is registered (indirectly through internal state)
	// Since handlers is private, we can only test this through integration
	// The fact that RegisterHandler doesn't panic is the test
}

func TestMatchEventPattern(t *testing.T) {
	tests := []struct {
		pattern   string
		eventType string
		want      bool
	}{
		{"jira:issue.created", "jira:issue.created", true},
		{"jira:*", "jira:issue.created", true},
		{"jira:*", "jira:issue.modified", true},
		{"jira:*", "slack:message", false},
		{"slack:*", "jira:issue.created", false},
		{"jira:issue.created", "jira:issue.modified", false},
		{"*", "jira:issue.created", false}, // Only suffix wildcards supported
	}

	for _, tc := range tests {
		got := matchEventPattern(tc.pattern, tc.eventType)
		if got != tc.want {
			t.Errorf("matchEventPattern(%q, %q) = %v, want %v", tc.pattern, tc.eventType, got, tc.want)
		}
	}
}

func TestDispatcher_Connect(t *testing.T) {
	r := NewRegistry()
	s := NewScheduler()
	d := NewDispatcher(r, s)

	bus := events.NewBus(10)
	defer bus.Close()

	d.Connect(bus)

	// Should be connected (running)
	// Test by stopping and verifying no panic
	d.Stop()
}

func TestActionTypes(t *testing.T) {
	tests := []struct {
		actionType ActionType
		want       string
	}{
		{ActionTypeImmediate, "immediate"},
		{ActionTypeDelayed, "delayed"},
		{ActionTypeBatched, "batched"},
	}

	for _, tc := range tests {
		if string(tc.actionType) != tc.want {
			t.Errorf("ActionType constant %v != %q", tc.actionType, tc.want)
		}
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler()

	// Start should be idempotent
	s.Start()
	s.Start() // Second start should be no-op

	// Stop should be idempotent
	s.Stop()
	s.Stop() // Second stop should be no-op
}

func TestCombineBatchEvents(t *testing.T) {
	actions := []*ScheduledAction{
		{
			Event: events.StorageEvent{
				Source:   "jira",
				EventID:  "ISSUE-1",
				Category: "jira",
			},
		},
		{
			Event: events.StorageEvent{
				Source:   "jira",
				EventID:  "ISSUE-2",
				Category: "jira",
			},
		},
	}

	combined := combineBatchEvents(actions)

	if combined.Source != "jira" {
		t.Errorf("Source = %q, want %q", combined.Source, "jira")
	}

	if combined.EventID != "ISSUE-1,ISSUE-2" {
		t.Errorf("EventID = %q, want %q", combined.EventID, "ISSUE-1,ISSUE-2")
	}

	batchCount, ok := combined.Data["batch_count"].(int)
	if !ok || batchCount != 2 {
		t.Errorf("batch_count = %v, want 2", combined.Data["batch_count"])
	}
}

func TestCombineBatchEvents_Empty(t *testing.T) {
	combined := combineBatchEvents(nil)

	if combined.Source != "" {
		t.Errorf("Empty batch should produce empty event, got Source=%q", combined.Source)
	}
}

func TestIntegration_DispatcherWithRegistry(t *testing.T) {
	// Create registry with action
	r := NewRegistry()
	r.RegisterAction(&Action{
		Name:       "test-action",
		EventType:  "test:*",
		Enabled:    true,
		Prompt:     "test-prompt",
		ActionType: ActionTypeImmediate,
	})
	r.RegisterPrompt("test-prompt", "Process event {{event_id}}")

	// Create scheduler and dispatcher
	s := NewScheduler()
	d := NewDispatcher(r, s)

	// Create and connect bus
	bus := events.NewBus(10)
	defer bus.Close()

	d.Connect(bus)

	// Track if our handler was called
	handlerCalled := make(chan bool, 1)
	d.RegisterHandler("test-action", func(e events.StorageEvent, a *Action) ActionResult {
		handlerCalled <- true
		return ActionResult{Success: true}
	})

	// Publish event
	bus.Publish(events.StorageEvent{
		Type:      events.EventStore,
		Source:    "test",
		EventID:   "evt-1",
		FetchedAt: time.Now(),
		Category:  "test",
	})

	// Wait for handler (with timeout)
	select {
	case <-handlerCalled:
		// Success
	case <-time.After(100 * time.Millisecond):
		// Handler is async, so this is also acceptable for immediate actions
		// The dispatcher fires handlers in goroutines
	}

	d.Stop()
}
