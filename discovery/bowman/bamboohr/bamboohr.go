// Package bamboohr provides on-demand fetching of BambooHR data for HAL 9000.
// "I'm sorry, Dave. I'm afraid I can't let you miss your one-on-one."
//
// This package fetches detailed employee profiles and stores them via bowman.
// For continuous monitoring, use floyd-bamboohr instead.
package bamboohr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pearcec/hal9000/discovery/bowman"
	"github.com/pearcec/hal9000/discovery/config"
)

// Config holds BambooHR connection settings.
type Config struct {
	Subdomain string `json:"subdomain"` // e.g., "yourcompany" (for yourcompany.bamboohr.com)
	APIKey    string `json:"api_key"`   // BambooHR API key
}

// Employee represents a BambooHR employee with full details.
type Employee struct {
	ID              string `json:"id"`
	EmployeeNumber  string `json:"employeeNumber,omitempty"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	PreferredName   string `json:"preferredName,omitempty"`
	DisplayName     string `json:"displayName"`
	JobTitle        string `json:"jobTitle,omitempty"`
	WorkEmail       string `json:"workEmail,omitempty"`
	Department      string `json:"department,omitempty"`
	Division        string `json:"division,omitempty"`
	Location        string `json:"location,omitempty"`
	Supervisor      string `json:"supervisor,omitempty"`
	SupervisorID    string `json:"supervisorId,omitempty"`
	SupervisorEmail string `json:"supervisorEmail,omitempty"`
	PhotoURL        string `json:"photoUrl,omitempty"`
	WorkPhone       string `json:"workPhone,omitempty"`
	MobilePhone     string `json:"mobilePhone,omitempty"`
	HomeEmail       string `json:"homeEmail,omitempty"`
	HireDate        string `json:"hireDate,omitempty"`
	Status          string `json:"status,omitempty"`
	// Additional profile fields
	BestEmail    string `json:"bestEmail,omitempty"`
	LinkedInURL  string `json:"linkedIn,omitempty"`
	WorkPhoneExt string `json:"workPhoneExtension,omitempty"`
	Address1     string `json:"address1,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	Country      string `json:"country,omitempty"`
	PostalCode   string `json:"zipcode,omitempty"`
}

// Client is a BambooHR API client.
type Client struct {
	config Config
	client *http.Client
}

// NewClient creates a new BambooHR client.
func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewClientFromConfig creates a client from the credentials file.
func NewClientFromConfig() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return NewClient(*cfg), nil
}

// LoadConfig loads BambooHR configuration from the credentials file.
func LoadConfig() (*Config, error) {
	path := filepath.Join(config.GetCredentialsDir(), "bamboohr-credentials.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read BambooHR config: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to parse BambooHR config: %v", err)
	}

	if cfg.Subdomain == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("BambooHR config missing subdomain or api_key")
	}

	return &cfg, nil
}

// GetEmployee fetches a single employee by ID with detailed profile fields.
func (c *Client) GetEmployee(employeeID string) (*Employee, error) {
	// BambooHR API: GET /v1/employees/{id}/?fields=...
	fields := "id,employeeNumber,firstName,lastName,preferredName,displayName,jobTitle," +
		"workEmail,department,division,location,supervisor,supervisorId,supervisorEmail," +
		"photoUrl,workPhone,mobilePhone,homeEmail,hireDate,status,bestEmail,linkedIn," +
		"workPhoneExtension,address1,city,state,country,zipcode"

	url := fmt.Sprintf("https://api.bamboohr.com/api/gateway.php/%s/v1/employees/%s/?fields=%s",
		c.config.Subdomain, employeeID, fields)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.config.APIKey + ":x"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("BambooHR request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BambooHR returned %d: %s", resp.StatusCode, string(body))
	}

	var employee Employee
	if err := json.NewDecoder(resp.Body).Decode(&employee); err != nil {
		return nil, fmt.Errorf("failed to parse employee response: %v", err)
	}

	return &employee, nil
}

// GetEmployeeDirectory fetches the employee directory (lightweight view).
func (c *Client) GetEmployeeDirectory() ([]Employee, error) {
	url := fmt.Sprintf("https://api.bamboohr.com/api/gateway.php/%s/v1/employees/directory",
		c.config.Subdomain)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.config.APIKey + ":x"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("BambooHR request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BambooHR returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Employees []Employee `json:"employees"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse directory response: %v", err)
	}

	return result.Employees, nil
}

// FetchAndStoreEmployee fetches an employee and stores it in the library.
func (c *Client) FetchAndStoreEmployee(employeeID string) (string, error) {
	employee, err := c.GetEmployee(employeeID)
	if err != nil {
		return "", err
	}

	return StoreEmployee(employee)
}

// StoreEmployee stores an employee profile in the library via bowman.
func StoreEmployee(employee *Employee) (string, error) {
	rawEvent := bowman.RawEvent{
		Source:    "bamboohr",
		EventID:   employee.ID,
		FetchedAt: time.Now(),
		Stage:     "raw",
		Data: map[string]interface{}{
			"id":              employee.ID,
			"employeeNumber":  employee.EmployeeNumber,
			"firstName":       employee.FirstName,
			"lastName":        employee.LastName,
			"preferredName":   employee.PreferredName,
			"displayName":     employee.DisplayName,
			"jobTitle":        employee.JobTitle,
			"workEmail":       employee.WorkEmail,
			"department":      employee.Department,
			"division":        employee.Division,
			"location":        employee.Location,
			"supervisor":      employee.Supervisor,
			"supervisorId":    employee.SupervisorID,
			"supervisorEmail": employee.SupervisorEmail,
			"photoUrl":        employee.PhotoURL,
			"workPhone":       employee.WorkPhone,
			"mobilePhone":     employee.MobilePhone,
			"homeEmail":       employee.HomeEmail,
			"hireDate":        employee.HireDate,
			"status":          employee.Status,
			"bestEmail":       employee.BestEmail,
			"linkedIn":        employee.LinkedInURL,
			"workPhoneExt":    employee.WorkPhoneExt,
			"address1":        employee.Address1,
			"city":            employee.City,
			"state":           employee.State,
			"country":         employee.Country,
			"postalCode":      employee.PostalCode,
		},
	}

	storeConfig := bowman.StoreConfig{
		LibraryPath: config.GetLibraryPath(),
		Category:    "bamboohr",
	}

	return bowman.Store(storeConfig, rawEvent)
}
