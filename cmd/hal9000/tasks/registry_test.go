package tasks

import (
	"context"
	"testing"
)

func TestRegistry(t *testing.T) {
	// Clear registry for test isolation
	registryMu.Lock()
	registry = make(map[string]Task)
	registryMu.Unlock()

	// Test empty registry
	if Count() != 0 {
		t.Errorf("Count() = %d, want 0", Count())
	}

	// Test Register
	task1 := &mockTask{name: "task1", description: "First task"}
	Register(task1)

	if Count() != 1 {
		t.Errorf("Count() = %d, want 1", Count())
	}

	// Test Get
	got := Get("task1")
	if got == nil {
		t.Fatal("Get(\"task1\") returned nil")
	}
	if got.Name() != "task1" {
		t.Errorf("Get(\"task1\").Name() = %q, want %q", got.Name(), "task1")
	}

	// Test Get non-existent
	if Get("nonexistent") != nil {
		t.Error("Get(\"nonexistent\") should return nil")
	}

	// Test duplicate registration panics
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register should panic on duplicate")
		}
	}()
	Register(task1)
}

func TestRegistryList(t *testing.T) {
	// Clear registry for test isolation
	registryMu.Lock()
	registry = make(map[string]Task)
	registryMu.Unlock()

	// Register tasks out of order
	Register(&mockTask{name: "zebra", description: "Z task"})
	Register(&mockTask{name: "alpha", description: "A task"})
	Register(&mockTask{name: "middle", description: "M task"})

	// Test List returns sorted
	tasks := List()
	if len(tasks) != 3 {
		t.Fatalf("List() returned %d tasks, want 3", len(tasks))
	}

	if tasks[0].Name() != "alpha" {
		t.Errorf("tasks[0].Name() = %q, want %q", tasks[0].Name(), "alpha")
	}
	if tasks[1].Name() != "middle" {
		t.Errorf("tasks[1].Name() = %q, want %q", tasks[1].Name(), "middle")
	}
	if tasks[2].Name() != "zebra" {
		t.Errorf("tasks[2].Name() = %q, want %q", tasks[2].Name(), "zebra")
	}
}

func TestRegistryNames(t *testing.T) {
	// Clear registry for test isolation
	registryMu.Lock()
	registry = make(map[string]Task)
	registryMu.Unlock()

	Register(&mockTask{name: "zebra", description: "Z task"})
	Register(&mockTask{name: "alpha", description: "A task"})

	names := Names()
	if len(names) != 2 {
		t.Fatalf("Names() returned %d names, want 2", len(names))
	}

	if names[0] != "alpha" {
		t.Errorf("names[0] = %q, want %q", names[0], "alpha")
	}
	if names[1] != "zebra" {
		t.Errorf("names[1] = %q, want %q", names[1], "zebra")
	}
}

// mockTask for registry tests (defined in task_test.go but repeated here for clarity)
func init() {
	// mockTask is defined in task_test.go
}

// Ensure mockTask implements Task
var _ Task = (*mockTask)(nil)

func newMockTask(name string) *mockTask {
	return &mockTask{
		name:        name,
		description: "Mock " + name,
		runFunc: func(ctx context.Context, opts RunOptions) (*Result, error) {
			return &Result{Success: true}, nil
		},
	}
}
