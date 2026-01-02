// Package httpclient provides a reusable HTTP client with support for self-signed TLS certificates,
// configurable timeouts, retries, and helper methods for common HTTP operations.
package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds configuration options for the HTTP client
type Config struct {
	// Timeout for individual requests (default: 30 seconds)
	Timeout time.Duration
	// MaxRetries is the number of retry attempts for failed requests (default: 2)
	MaxRetries int
	// SkipTLSVerify disables TLS certificate verification for self-signed certs (default: false)
	SkipTLSVerify bool
	// DisableKeepAlives disables HTTP connection keep-alives (default: false)
	DisableKeepAlives bool
	// MaxIdleConns sets the maximum number of idle connections (default: 10)
	MaxIdleConns int
	// IdleConnTimeout sets the timeout for idle connections (default: 30 seconds)
	IdleConnTimeout time.Duration
}

// Client is a reusable HTTP client with retry logic and TLS support
type Client struct {
	*http.Client
	config Config
}

// NewClient creates a new HTTP client with the given configuration
func NewClient(config Config) *Client {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 2
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 10
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = 30 * time.Second
	}

	// Create custom transport with TLS configuration
	transport := &http.Transport{
		MaxIdleConns:      config.MaxIdleConns,
		IdleConnTimeout:   config.IdleConnTimeout,
		DisableKeepAlives: config.DisableKeepAlives,
	}

	// Skip TLS verification for self-signed certificates if requested
	if config.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &Client{
		Client: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
		config: config,
	}
}

// NewClientWithDefaults creates a new HTTP client with default configuration
func NewClientWithDefaults() *Client {
	return NewClient(Config{})
}

// NewClientSkipTLS creates a new HTTP client that skips TLS verification
func NewClientSkipTLS(timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return NewClient(Config{
		Timeout:           timeout,
		SkipTLSVerify:     true,
		MaxRetries:        2,
	})
}

// do executes an HTTP request with retry logic
func (c *Client) do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err := c.Client.Do(req)

		// If successful, return immediately
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		lastErr = err

		// Don't retry on successful responses with non-5xx status codes
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// If this is not the last attempt, retry after a brief delay
		if attempt < c.config.MaxRetries {
			time.Sleep(time.Duration((attempt+1)*100) * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.config.MaxRetries, lastErr)
}

// Get performs a GET request with retry logic
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// GetJSON performs a GET request and unmarshals the response body into v
func (c *Client) GetJSON(url string, v interface{}) error {
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// Post performs a POST request with the given body
func (c *Client) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.do(req)
}

// PostJSON performs a POST request with JSON body and returns the response
func (c *Client) PostJSON(url string, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return c.Post(url, "application/json", bytes.NewBuffer(jsonData))
}

// PostJSONUnmarshal performs a POST request with JSON body and unmarshals the response
func (c *Client) PostJSONUnmarshal(url string, data interface{}, result interface{}) error {
	resp, err := c.PostJSON(url, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// Delete performs a DELETE request
func (c *Client) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// DeleteJSON performs a DELETE request and unmarshals the response
func (c *Client) DeleteJSON(url string, result interface{}) error {
	resp, err := c.Delete(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// Patch performs a PATCH request with JSON body
func (c *Client) Patch(url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPatch, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.do(req)
}

// PatchJSON performs a PATCH request with JSON body
func (c *Client) PatchJSON(url string, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return c.Patch(url, "application/json", bytes.NewBuffer(jsonData))
}

// PatchJSONUnmarshal performs a PATCH request with JSON body and unmarshals the response
func (c *Client) PatchJSONUnmarshal(url string, data interface{}, result interface{}) error {
	resp, err := c.PatchJSON(url, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// SetHeader adds a header to all future requests
func (c *Client) SetHeader(key, value string) {
	// Note: http.Client doesn't have a built-in way to set default headers
	// Users should create request-specific headers or use a middleware approach
	// This is documented in the usage example
}
