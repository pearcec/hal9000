package poole

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Registry manages action definitions and prompt templates.
// Actions define what to do when events occur; prompts define how to
// instruct Claude for each action type.
type Registry struct {
	actions      map[string]*Action           // actionName -> Action
	eventIndex   map[string][]*Action         // eventType -> Actions
	prompts      map[string]string            // promptName -> template content
	promptPaths  []string                     // directories to search for prompts
	mu           sync.RWMutex
}

// ActionsConfig is the YAML structure for actions configuration.
type ActionsConfig struct {
	Actions map[string]ActionConfig `yaml:"actions"`
}

// ActionConfig is the YAML structure for a single action.
type ActionConfig struct {
	Enabled    bool              `yaml:"enabled"`
	EventType  string            `yaml:"event_type"`
	Fetchers   []string          `yaml:"fetch"`
	Prompt     string            `yaml:"prompt"`
	ActionType string            `yaml:"action_type"`
	Metadata   map[string]string `yaml:"metadata"`
}

// NewRegistry creates a new action and prompt registry.
func NewRegistry() *Registry {
	return &Registry{
		actions:     make(map[string]*Action),
		eventIndex:  make(map[string][]*Action),
		prompts:     make(map[string]string),
		promptPaths: make([]string, 0),
	}
}

// LoadActions loads action definitions from a YAML file.
// Actions define which events trigger which prompts and fetchers.
func (r *Registry) LoadActions(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read actions file: %w", err)
	}

	var config ActionsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse actions YAML: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for name, cfg := range config.Actions {
		actionType := ActionTypeImmediate
		switch cfg.ActionType {
		case "delayed":
			actionType = ActionTypeDelayed
		case "batched":
			actionType = ActionTypeBatched
		}

		action := &Action{
			Name:       name,
			EventType:  cfg.EventType,
			Enabled:    cfg.Enabled,
			Fetchers:   cfg.Fetchers,
			Prompt:     cfg.Prompt,
			ActionType: actionType,
			Metadata:   cfg.Metadata,
		}

		r.actions[name] = action

		// Index by event type for fast lookup
		r.eventIndex[cfg.EventType] = append(r.eventIndex[cfg.EventType], action)
	}

	return nil
}

// RegisterAction adds an action to the registry.
func (r *Registry) RegisterAction(action *Action) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actions[action.Name] = action
	r.eventIndex[action.EventType] = append(r.eventIndex[action.EventType], action)
}

// GetAction retrieves an action by name.
func (r *Registry) GetAction(name string) (*Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	action, exists := r.actions[name]
	return action, exists
}

// GetActionsForEvent returns all actions registered for an event type.
func (r *Registry) GetActionsForEvent(eventType string) []*Action {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct match
	if actions, ok := r.eventIndex[eventType]; ok {
		return actions
	}

	// Wildcard match: check for "source:*" patterns
	var matched []*Action
	for key, actions := range r.eventIndex {
		if matchEventPattern(key, eventType) {
			matched = append(matched, actions...)
		}
	}

	return matched
}

// matchEventPattern checks if pattern matches the event type.
// Supports wildcards: "jira:*" matches "jira:issue.created"
func matchEventPattern(pattern, eventType string) bool {
	if pattern == eventType {
		return true
	}

	// Handle wildcard patterns
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		return strings.HasPrefix(eventType, prefix+":")
	}

	return false
}

// ListActions returns all registered actions, sorted by name.
func (r *Registry) ListActions() []*Action {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actions := make([]*Action, 0, len(r.actions))
	for _, action := range r.actions {
		actions = append(actions, action)
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})

	return actions
}

// AddPromptPath adds a directory to search for prompt templates.
// Paths are searched in order; later paths override earlier ones.
func (r *Registry) AddPromptPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptPaths = append(r.promptPaths, path)
}

// LoadPrompts loads all prompt templates from configured paths.
// Files should be named {prompt-name}.md in the prompt directories.
func (r *Registry) LoadPrompts() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, basePath := range r.promptPaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			// Skip directories that don't exist
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to read prompt directory %s: %w", basePath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".md")
			path := filepath.Join(basePath, entry.Name())

			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read prompt %s: %w", path, err)
			}

			// Later paths override earlier ones (user overrides defaults)
			r.prompts[name] = string(content)
		}
	}

	return nil
}

// RegisterPrompt adds a prompt template to the registry.
func (r *Registry) RegisterPrompt(name, template string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts[name] = template
}

// GetPrompt retrieves a prompt template by name.
func (r *Registry) GetPrompt(name string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, exists := r.prompts[name]
	if !exists {
		return "", fmt.Errorf("prompt not found: %s", name)
	}
	return template, nil
}

// ListPrompts returns the names of all registered prompts.
func (r *Registry) ListPrompts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.prompts))
	for name := range r.prompts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ExpandPrompt substitutes variables in a prompt template.
// Variables are in the form {{variable_name}}.
func ExpandPrompt(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
