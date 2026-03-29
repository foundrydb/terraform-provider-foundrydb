package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the HTTP client for the FoundryDB API.
type Client struct {
	apiURL     string
	username   string
	password   string
	httpClient *http.Client
}

// New creates a new FoundryDB API client.
func New(apiURL, username, password string) *Client {
	return &Client{
		apiURL:   strings.TrimRight(apiURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.apiURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CreateService creates a new managed database service.
func (c *Client) CreateService(req ServiceCreateRequest) (*Service, error) {
	resp, err := c.doRequest(http.MethodPost, "/managed-services/", req)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create service returned status %d: %s", resp.StatusCode, string(data))
	}
	var svc Service
	if err := json.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("failed to decode service response: %w", err)
	}
	return &svc, nil
}

// GetService retrieves a managed database service by ID.
func (c *Client) GetService(id string) (*Service, error) {
	resp, err := c.doRequest(http.MethodGet, "/managed-services/"+id, nil)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get service returned status %d: %s", resp.StatusCode, string(data))
	}
	var svc Service
	if err := json.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("failed to decode service response: %w", err)
	}
	return &svc, nil
}

// UpdateService updates a managed database service.
func (c *Client) UpdateService(id string, req ServiceUpdateRequest) (*Service, error) {
	resp, err := c.doRequest(http.MethodPatch, "/managed-services/"+id, req)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update service returned status %d: %s", resp.StatusCode, string(data))
	}
	var svc Service
	if err := json.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("failed to decode service response: %w", err)
	}
	return &svc, nil
}

// DeleteService deletes a managed database service.
func (c *Client) DeleteService(id string) error {
	resp, err := c.doRequest(http.MethodDelete, "/managed-services/"+id, nil)
	if err != nil {
		return err
	}
	data, err := readBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete service returned status %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// WaitForServiceRunning polls until the service reaches "running" status or the
// timeout is exceeded. Polling interval is 10 seconds; timeout is 15 minutes.
func (c *Client) WaitForServiceRunning(id string) (*Service, error) {
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		svc, err := c.GetService(id)
		if err != nil {
			return nil, fmt.Errorf("error polling service status: %w", err)
		}
		if svc == nil {
			return nil, fmt.Errorf("service %s not found while waiting for running status", id)
		}
		if strings.EqualFold(svc.Status, "running") {
			return svc, nil
		}
		// Surface terminal failure states immediately rather than waiting out the timeout.
		if strings.EqualFold(svc.Status, "failed") || strings.EqualFold(svc.Status, "error") {
			return nil, fmt.Errorf("service %s entered terminal status: %s", id, svc.Status)
		}
		time.Sleep(10 * time.Second)
	}
	return nil, fmt.Errorf("timed out after 15 minutes waiting for service %s to reach running status", id)
}

// RevealDatabaseUserPassword fetches the credentials for a database user.
func (c *Client) RevealDatabaseUserPassword(serviceID, username string) (*DatabaseUser, error) {
	path := fmt.Sprintf("/managed-services/%s/database-users/%s/reveal-password", serviceID, username)
	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reveal password returned status %d: %s", resp.StatusCode, string(data))
	}
	var user DatabaseUser
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}
	return &user, nil
}
