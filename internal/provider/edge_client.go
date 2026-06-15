package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// edgeClient is a minimal HTTP client for the Edge Gateway API surface that is
// not yet present in the vendored foundrydb-sdk-go. It uses the same Basic Auth
// + Content-Type conventions as the SDK client.
type edgeClient struct {
	apiURL     string
	username   string
	password   string
	httpClient *http.Client
}

// newEdgeClient constructs an edgeClient from the provider config fields.
func newEdgeClient(apiURL, username, password string) *edgeClient {
	u := strings.TrimRight(apiURL, "/")
	if u == "" {
		u = "https://api.foundrydb.com"
	}
	return &edgeClient{
		apiURL:   u,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// do executes an authenticated HTTP request and returns the raw response.
func (c *edgeClient) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("edge: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("edge: create request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("edge: request %s %s: %w", method, path, err)
	}
	return resp, nil
}

// checkResp reads the response body. On a non-2xx status it returns an error
// with the status code and message; on success it returns the raw body bytes.
func (c *edgeClient) checkResp(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("edge: read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractEdgeErrorMessage(data)
		return nil, fmt.Errorf("edge: API error %d: %s", resp.StatusCode, msg)
	}
	return data, nil
}

func extractEdgeErrorMessage(body []byte) string {
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.Error != "" {
			return payload.Error
		}
		if payload.Message != "" {
			return payload.Message
		}
	}
	raw := strings.TrimSpace(string(body))
	if len(raw) > 200 {
		raw = raw[:200] + "..."
	}
	return raw
}

// EdgeDomain is a customer custom domain attached to an app service.
type EdgeDomain struct {
	ID          string `json:"id"`
	ServiceID   string `json:"service_id"`
	Domain      string `json:"domain"`
	Status      string `json:"status"`
	CNAMETarget string `json:"cname_target,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type createEdgeDomainRequest struct {
	Domain string `json:"domain"`
}

type listEdgeDomainsResponse struct {
	Domains []EdgeDomain `json:"domains"`
}

// CreateDomain attaches a custom domain to an app service.
func (c *edgeClient) CreateDomain(ctx context.Context, appServiceID string, domain string) (*EdgeDomain, error) {
	resp, err := c.do(ctx, http.MethodPost, "/app-services/"+appServiceID+"/domains", createEdgeDomainRequest{Domain: domain})
	if err != nil {
		return nil, err
	}
	data, err := c.checkResp(resp)
	if err != nil {
		return nil, err
	}
	var d EdgeDomain
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("edge: decode CreateDomain response: %w", err)
	}
	return &d, nil
}

// ListDomains returns all custom domains attached to an app service.
func (c *edgeClient) ListDomains(ctx context.Context, appServiceID string) ([]EdgeDomain, error) {
	resp, err := c.do(ctx, http.MethodGet, "/app-services/"+appServiceID+"/domains", nil)
	if err != nil {
		return nil, err
	}
	data, err := c.checkResp(resp)
	if err != nil {
		return nil, err
	}
	var result listEdgeDomainsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("edge: decode ListDomains response: %w", err)
	}
	return result.Domains, nil
}

// GetDomain finds a domain by ID within the app service's domain list. Returns
// nil when no domain with that ID exists (treats it as deleted).
func (c *edgeClient) GetDomain(ctx context.Context, appServiceID, domainID string) (*EdgeDomain, error) {
	domains, err := c.ListDomains(ctx, appServiceID)
	if err != nil {
		return nil, err
	}
	for i := range domains {
		if domains[i].ID == domainID {
			return &domains[i], nil
		}
	}
	return nil, nil
}

// DeleteDomain removes a custom domain from an app service. A 404 is treated
// as success (idempotent).
func (c *edgeClient) DeleteDomain(ctx context.Context, appServiceID, domainID string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/app-services/"+appServiceID+"/domains/"+domainID, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil
	}
	_, err = c.checkResp(resp)
	return err
}

// EdgeCacheRule caches responses under one path prefix for a fixed TTL.
type EdgeCacheRule struct {
	PathPrefix string `json:"path_prefix"`
	TTLSeconds int    `json:"ttl_seconds"`
}

// EdgeRateLimit is a token bucket enforced per PoP.
type EdgeRateLimit struct {
	RequestsPerSecond int    `json:"requests_per_second"`
	Burst             int    `json:"burst"`
	Key               string `json:"key"`
}

// EdgeSettingsRequest holds the customer-tunable edge settings.
type EdgeSettingsRequest struct {
	CacheRules []EdgeCacheRule `json:"cache_rules,omitempty"`
	RateLimit  *EdgeRateLimit  `json:"rate_limit,omitempty"`
	WAFMode    *string         `json:"waf_mode,omitempty"`
}

// EdgeSettings is the response from an edge settings update or read.
type EdgeSettings struct {
	CacheRules    []EdgeCacheRule `json:"cache_rules,omitempty"`
	RateLimit     *EdgeRateLimit  `json:"rate_limit,omitempty"`
	WAFMode       string          `json:"waf_mode"`
	ConfigVersion int64           `json:"config_version"`
}

// EdgeStatus is the edge overview for an app service.
type EdgeStatus struct {
	EdgeEnabled   bool          `json:"edge_enabled"`
	CNAMETarget   string        `json:"cname_target,omitempty"`
	ConfigVersion int64         `json:"config_version"`
	Settings      *EdgeSettings `json:"settings,omitempty"`
}

// GetEdgeStatus returns the edge status (and embedded settings) for an app service.
func (c *edgeClient) GetEdgeStatus(ctx context.Context, appServiceID string) (*EdgeStatus, error) {
	resp, err := c.do(ctx, http.MethodGet, "/app-services/"+appServiceID+"/edge", nil)
	if err != nil {
		return nil, err
	}
	data, err := c.checkResp(resp)
	if err != nil {
		return nil, err
	}
	var status EdgeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("edge: decode GetEdgeStatus response: %w", err)
	}
	return &status, nil
}

// UpdateEdgeSettings replaces the edge settings for an app service.
func (c *edgeClient) UpdateEdgeSettings(ctx context.Context, appServiceID string, req EdgeSettingsRequest) (*EdgeSettings, error) {
	resp, err := c.do(ctx, http.MethodPut, "/app-services/"+appServiceID+"/edge/settings", req)
	if err != nil {
		return nil, err
	}
	data, err := c.checkResp(resp)
	if err != nil {
		return nil, err
	}
	var settings EdgeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("edge: decode UpdateEdgeSettings response: %w", err)
	}
	return &settings, nil
}
