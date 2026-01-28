package tasks

import (
	"fmt"
	"sort"
	"sync"
)

// registry holds all registered tasks.
var (
	registryMu sync.RWMutex
	registry   = make(map[string]Task)
)

// Register adds a task to the registry.
// It panics if a task with the same name is already registered.
func Register(task Task) {
	registryMu.Lock()
	defer registryMu.Unlock()

	name := task.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("task already registered: %s", name))
	}
	registry[name] = task
}

// Get retrieves a task by name from the registry.
// Returns nil if no task with that name is registered.
func Get(name string) Task {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return registry[name]
}

// List returns all registered tasks, sorted by name.
func List() []Task {
	registryMu.RLock()
	defer registryMu.RUnlock()

	tasks := make([]Task, 0, len(registry))
	for _, task := range registry {
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name() < tasks[j].Name()
	})

	return tasks
}

// Names returns the names of all registered tasks, sorted alphabetically.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// Count returns the number of registered tasks.
func Count() int {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return len(registry)
}
