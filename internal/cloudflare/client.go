package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// Record contains the mutable fields Flare preserves when replacing an A or
// AAAA record through Cloudflare's update endpoint.
type Record struct {
	ID             string         `json:"id,omitempty"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Content        string         `json:"content"`
	TTL            int            `json:"ttl"`
	Proxied        *bool          `json:"proxied,omitempty"`
	Comment        string         `json:"comment,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Settings       map[string]any `json:"settings,omitempty"`
	PrivateRouting *bool          `json:"private_routing,omitempty"`
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type envelope[T any] struct {
	Result  T            `json:"result"`
	Success bool         `json:"success"`
	Errors  []apiMessage `json:"errors"`
}

// Client is a small client for the Cloudflare DNS Records API.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	APIToken   string
}

// NewClient creates a Cloudflare API client.
func NewClient(httpClient *http.Client, apiToken string) *Client {
	return &Client{HTTPClient: httpClient, BaseURL: defaultBaseURL, APIToken: apiToken}
}

// GetRecord fetches one DNS record by identifier.
func (c *Client) GetRecord(ctx context.Context, zoneID, recordID string) (Record, error) {
	var response envelope[Record]
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	if err := c.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return Record{}, err
	}
	return response.Result, nil
}

// FindRecord finds a unique DNS record by type and exact name.
func (c *Client) FindRecord(ctx context.Context, zoneID, recordType, name string) (Record, error) {
	query := url.Values{"type": {recordType}, "name": {name}}
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records?" + query.Encode()
	var response envelope[[]Record]
	if err := c.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return Record{}, err
	}
	if len(response.Result) == 0 {
		return Record{}, fmt.Errorf("DNS record %s %q was not found", recordType, name)
	}
	if len(response.Result) > 1 {
		return Record{}, fmt.Errorf("DNS record lookup returned %d records for %s %q", len(response.Result), recordType, name)
	}
	return response.Result[0], nil
}

// UpdateRecord overwrites a DNS record while preserving the mutable fields
// obtained from Cloudflare.
func (c *Client) UpdateRecord(ctx context.Context, zoneID string, record Record) (Record, error) {
	if record.ID == "" {
		return Record{}, errors.New("DNS record ID is empty")
	}

	body := struct {
		Type           string         `json:"type"`
		Name           string         `json:"name"`
		Content        string         `json:"content"`
		TTL            int            `json:"ttl"`
		Proxied        *bool          `json:"proxied,omitempty"`
		Comment        string         `json:"comment,omitempty"`
		Tags           []string       `json:"tags,omitempty"`
		Settings       map[string]any `json:"settings,omitempty"`
		PrivateRouting *bool          `json:"private_routing,omitempty"`
	}{
		Type: record.Type, Name: record.Name, Content: record.Content, TTL: record.TTL,
		Proxied: record.Proxied, Comment: record.Comment, Tags: record.Tags,
		Settings: record.Settings, PrivateRouting: record.PrivateRouting,
	}

	var response envelope[Record]
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(record.ID)
	if err := c.do(ctx, http.MethodPut, path, body, &response); err != nil {
		return Record{}, err
	}
	return response.Result, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, destination any) error {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Cloudflare request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, requestBody)
	if err != nil {
		return fmt.Errorf("create Cloudflare request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.APIToken)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("Cloudflare request: %w", err)
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, 1<<20)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("read Cloudflare response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API returned %s: %s", response.Status, apiError(payload))
	}
	if err := json.Unmarshal(payload, destination); err != nil {
		return fmt.Errorf("decode Cloudflare response: %w", err)
	}

	success := struct {
		Success bool         `json:"success"`
		Errors  []apiMessage `json:"errors"`
	}{}
	if err := json.Unmarshal(payload, &success); err != nil {
		return fmt.Errorf("decode Cloudflare response status: %w", err)
	}
	if !success.Success {
		return fmt.Errorf("Cloudflare API rejected the request: %s", formatMessages(success.Errors))
	}
	return nil
}

func apiError(payload []byte) string {
	var response struct {
		Errors []apiMessage `json:"errors"`
	}
	if json.Unmarshal(payload, &response) == nil && len(response.Errors) > 0 {
		return formatMessages(response.Errors)
	}
	message := strings.TrimSpace(string(payload))
	if message == "" {
		return "empty response"
	}
	return message
}

func formatMessages(messages []apiMessage) string {
	if len(messages) == 0 {
		return "unknown error"
	}
	formatted := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.Code == 0 {
			formatted = append(formatted, message.Message)
		} else {
			formatted = append(formatted, fmt.Sprintf("%d: %s", message.Code, message.Message))
		}
	}
	return strings.Join(formatted, "; ")
}
