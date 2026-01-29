// Package main implements the BambooHR Floyd (watcher) dog for HAL 9000.
// "I know that you and Frank were planning to disconnect me, and I'm afraid
// that's something I cannot allow to happen."
//
// Floyd watches for BambooHR notifications and employee changes.
// Named after Dr. Heywood Floyd, the observer/investigator from 2001.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/discovery/config"
	evbus "github.com/pearcec/hal9000/discovery/events"
)

const (
	pollInterval = 5 * time.Minute
)

// getConfigPath returns the path to BambooHR credentials
func getConfigPath() string {
	return filepath.Join(config.GetCredentialsDir(), "bamboohr-credentials.yaml")
}

// getStatePath returns the path to BambooHR state file
func getStatePath() string {
	return filepath.Join(config.GetRuntimeDir(), "bamboohr-floyd-state.json")
}

// getEventsPath returns the path to BambooHR events file
func getEventsPath() string {
	return filepath.Join(config.GetRuntimeDir(), "bamboohr-events.jsonl")
}

// Config holds BambooHR connection settings.
type Config struct {
	Subdomain string `json:"subdomain"` // e.g., "yourcompany" (for yourcompany.bamboohr.com)
	APIKey    string `json:"api_key"`   // BambooHR API key
}

// Event represents a BambooHR change event emitted by Floyd (watcher).
type Event struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"` // Employee ID or notification ID
	Timestamp time.Time `json:"timestamp"`
}

// FloydState tracks known employees and notifications to detect changes.
type FloydState struct {
	Employees map[string]string `json:"employees"` // employeeID -> hash of employee data
	UpdatedAt time.Time         `json:"updated_at"`
}

// BambooEmployee represents an employee from the BambooHR API.
type BambooEmployee struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	PreferredName  string `json:"preferredName,omitempty"`
	JobTitle       string `json:"jobTitle"`
	WorkEmail      string `json:"workEmail"`
	Department     string `json:"department"`
	Division       string `json:"division,omitempty"`
	Location       string `json:"location,omitempty"`
	Supervisor     string `json:"supervisor,omitempty"`
	SupervisorID   string `json:"supervisorId,omitempty"`
	PhotoURL       string `json:"photoUrl,omitempty"`
	WorkPhone      string `json:"workPhone,omitempty"`
	MobilePhone    string `json:"mobilePhone,omitempty"`
	HireDate       string `json:"hireDate,omitempty"`
	Status         string `json:"status,omitempty"`
}

// BambooDirectoryResponse is the BambooHR employee directory response.
type BambooDirectoryResponse struct {
	Employees []BambooEmployee `json:"employees"`
	Fields    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"fields"`
}

// eventBus is the event bus for storage operations.
// Floyd emits events here; storage subscriber handles persistence.
var eventBus *evbus.Bus

func main() {
	log.Println("[floyd][watcher] HAL 9000 BambooHR Floyd initializing...")

	// Initialize event bus with storage handler
	eventBus = evbus.NewBus(100)
	eventBus.Subscribe(evbus.StorageHandler(config.GetLibraryPath()))
	defer eventBus.Close()

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config: %v", err)
	}

	log.Printf("[floyd][watcher] BambooHR Floyd online. Watching: %s.bamboohr.com", cfg.Subdomain)

	// Load or initialize state
	state := loadState()

	// Run watch loop
	for {
		changeEvents, newState, err := watchBambooHR(cfg, state)
		if err != nil {
			log.Printf("Error watching BambooHR: %v", err)
		} else {
			for _, event := range changeEvents {
				emitEvent(event)
			}
			state = newState
			saveState(state)
		}

		time.Sleep(pollInterval)
	}
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// loadConfig loads BambooHR configuration from file.
func loadConfig() (*Config, error) {
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file %s: %v\n\nCreate config with:\n%s", path, err, configExample())
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to parse config: %v", err)
	}

	if config.Subdomain == "" || config.APIKey == "" {
		return nil, fmt.Errorf("config missing required fields (subdomain, api_key)")
	}

	return &config, nil
}

func configExample() string {
	return `{
  "subdomain": "yourcompany",
  "api_key": "your-api-key"
}`
}

// watchBambooHR checks for employee directory changes and returns events.
func watchBambooHR(config *Config, state FloydState) ([]Event, FloydState, error) {
	var events []Event
	newState := FloydState{
		Employees: make(map[string]string),
		UpdatedAt: time.Now(),
	}

	// Query BambooHR employee directory
	employees, err := getEmployeeDirectory(config)
	if err != nil {
		return nil, state, err
	}

	// Build new state and detect changes
	currentIDs := make(map[string]bool)
	for _, employee := range employees {
		hash := hashEmployee(employee)
		newState.Employees[employee.ID] = hash
		currentIDs[employee.ID] = true

		oldHash, existed := state.Employees[employee.ID]
		if !existed {
			// New employee - store via Bowman
			if err := storeEmployee(employee); err != nil {
				log.Printf("Error storing new employee: %v", err)
			}
			events = append(events, Event{
				Source:    "bamboohr",
				Type:      "employee.new",
				Payload:   employee.ID,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] New employee detected: %s - %s", employee.ID, employee.DisplayName)
		} else if oldHash != hash {
			// Modified employee - update via Bowman
			if err := storeEmployee(employee); err != nil {
				log.Printf("Error updating employee: %v", err)
			}
			events = append(events, Event{
				Source:    "bamboohr",
				Type:      "employee.modified",
				Payload:   employee.ID,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Modified employee detected: %s - %s", employee.ID, employee.DisplayName)
		}
	}

	// Detect employees no longer in directory (may have left company)
	for id := range state.Employees {
		if !currentIDs[id] {
			events = append(events, Event{
				Source:    "bamboohr",
				Type:      "employee.removed",
				Payload:   id,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Employee removed from directory: %s", id)
		}
	}

	return events, newState, nil
}

// getEmployeeDirectory fetches the employee directory from BambooHR.
func getEmployeeDirectory(config *Config) ([]BambooEmployee, error) {
	url := fmt.Sprintf("https://api.bamboohr.com/api/gateway.php/%s/v1/employees/directory",
		config.Subdomain)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Basic auth with api_key:x (BambooHR uses API key as username, 'x' as password)
	auth := base64.StdEncoding.EncodeToString([]byte(config.APIKey + ":x"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("BambooHR request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BambooHR returned %d: %s", resp.StatusCode, string(body))
	}

	var result BambooDirectoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse BambooHR response: %v", err)
	}

	return result.Employees, nil
}

// storeEmployee emits a storage event for a BambooHR employee.
func storeEmployee(employee BambooEmployee) error {
	storageEvent := evbus.StorageEvent{
		Type:      evbus.EventStore,
		Source:    "bamboohr",
		EventID:   employee.ID,
		FetchedAt: time.Now(),
		Category:  "bamboohr",
		Data: map[string]interface{}{
			"id":            employee.ID,
			"displayName":   employee.DisplayName,
			"firstName":     employee.FirstName,
			"lastName":      employee.LastName,
			"preferredName": employee.PreferredName,
			"jobTitle":      employee.JobTitle,
			"workEmail":     employee.WorkEmail,
			"department":    employee.Department,
			"division":      employee.Division,
			"location":      employee.Location,
			"supervisor":    employee.Supervisor,
			"supervisorId":  employee.SupervisorID,
			"photoUrl":      employee.PhotoURL,
			"workPhone":     employee.WorkPhone,
			"mobilePhone":   employee.MobilePhone,
			"hireDate":      employee.HireDate,
			"status":        employee.Status,
		},
	}

	result := eventBus.Publish(storageEvent)
	return result.Error
}

// hashEmployee creates a hash of employee data to detect changes.
func hashEmployee(employee BambooEmployee) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		employee.ID,
		employee.DisplayName,
		employee.JobTitle,
		employee.Department,
		employee.WorkEmail,
		employee.Supervisor,
		employee.Location,
		employee.Status,
	)
}

// loadState loads Floyd state from disk.
func loadState() FloydState {
	path := getStatePath()
	state := FloydState{Employees: make(map[string]string)}

	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

// saveState persists Floyd state to disk.
func saveState(state FloydState) {
	path := getStatePath()
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, data, 0644)
}

// emitEvent outputs an event.
func emitEvent(event Event) {
	data, _ := json.Marshal(event)
	log.Printf("[floyd][watcher] EVENT: %s", string(data))

	// Write to events file
	path := getEventsPath()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Unable to write event: %v", err)
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
