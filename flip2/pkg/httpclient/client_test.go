package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client creation with custom configuration
func TestNewClient(t *testing.T) {
	config := Config{
		Timeout:        15 * time.Second,
		MaxRetries:     3,
		SkipTLSVerify:  true,
		MaxIdleConns:   20,
		IdleConnTimeout: 45 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
	assert.Equal(t, 15*time.Second, client.Timeout)
	assert.Equal(t, 3, client.config.MaxRetries)
	assert.True(t, client.config.SkipTLSVerify)
}

// TestNewClientDefaults tests that default values are set correctly
func TestNewClientDefaults(t *testing.T) {
	config := Config{}
	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.Equal(t, 2, client.config.MaxRetries)
	assert.False(t, client.config.SkipTLSVerify)
}

// TestNewClientWithDefaults tests the convenience constructor
func TestNewClientWithDefaults(t *testing.T) {
	client := NewClientWithDefaults()

	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.Equal(t, 2, client.config.MaxRetries)
}

// TestNewClientSkipTLS tests the TLS skip convenience constructor
func TestNewClientSkipTLS(t *testing.T) {
	client := NewClientSkipTLS(15 * time.Second)

	assert.NotNil(t, client)
	assert.Equal(t, 15*time.Second, client.Timeout)
	assert.True(t, client.config.SkipTLSVerify)
	assert.Equal(t, 2, client.config.MaxRetries)
}

// TestTLSConfiguration tests that TLS is properly configured
func TestTLSConfiguration(t *testing.T) {
	config := Config{
		SkipTLSVerify: true,
	}
	client := NewClient(config)

	transport := client.Transport.(*http.Transport)
	assert.NotNil(t, transport.TLSClientConfig)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

// TestGetRequest tests basic GET request
func TestGetRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	resp, err := client.Get(server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))
}

// TestGetJSON tests GET request with JSON response
func TestGetJSON(t *testing.T) {
	data := map[string]string{"name": "test", "value": "data"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	var result map[string]string
	err := client.GetJSON(server.URL, &result)

	require.NoError(t, err)
	assert.Equal(t, data, result)
}

// TestPostJSON tests POST request with JSON body
func TestPostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var data map[string]string
		json.NewDecoder(r.Body).Decode(&data)
		assert.Equal(t, "test", data["name"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	resp, err := client.PostJSON(server.URL, map[string]string{"name": "test"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestPostJSONUnmarshal tests POST with automatic response unmarshaling
func TestPostJSONUnmarshal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123", "status": "success"})
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	var result map[string]string
	err := client.PostJSONUnmarshal(server.URL, map[string]string{"name": "test"}, &result)

	require.NoError(t, err)
	assert.Equal(t, "123", result["id"])
	assert.Equal(t, "success", result["status"])
}

// TestDeleteRequest tests DELETE request
func TestDeleteRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	resp, err := client.Delete(server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestDeleteJSON tests DELETE request with JSON response
func TestDeleteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"deleted": "true"})
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	var result map[string]string
	err := client.DeleteJSON(server.URL, &result)

	require.NoError(t, err)
	assert.Equal(t, "true", result["deleted"])
}

// TestPatchJSON tests PATCH request with JSON body
func TestPatchJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var data map[string]string
		json.NewDecoder(r.Body).Decode(&data)
		assert.Equal(t, "updated", data["name"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": "updated"})
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	resp, err := client.PatchJSON(server.URL, map[string]string{"name": "updated"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestPatchJSONUnmarshal tests PATCH with automatic response unmarshaling
func TestPatchJSONUnmarshal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"updated": "true", "id": "456"})
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	var result map[string]string
	err := client.PatchJSONUnmarshal(server.URL, map[string]string{"status": "active"}, &result)

	require.NoError(t, err)
	assert.Equal(t, "true", result["updated"])
	assert.Equal(t, "456", result["id"])
}

// TestRetryLogic tests that requests are retried on 5xx errors
func TestRetryLogic(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// Return 500 error for first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Return success on third attempt
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		MaxRetries: 2,
	})

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, callCount)
}

// TestRetryFailure tests that request fails after max retries
func TestRetryFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{
		MaxRetries: 2,
	})

	resp, _ := client.Get(server.URL)
	// Should still get response, but with error status
	if resp != nil {
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	}
	// Called: initial + 2 retries = 3 times
	assert.Equal(t, 3, callCount)
}

// TestTimeoutHandling tests that timeout is respected
func TestTimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with very short timeout
	client := NewClient(Config{
		Timeout: 10 * time.Millisecond,
	})

	_, err := client.Get(server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// TestErrorResponses tests error handling for various HTTP status codes
func TestErrorResponses(t *testing.T) {
	testCases := []struct {
		statusCode int
		method     string
		testFunc   func(*Client, string) error
	}{
		{
			statusCode: http.StatusBadRequest,
			method:     "GET",
			testFunc: func(c *Client, url string) error {
				var result map[string]string
				return c.GetJSON(url, &result)
			},
		},
		{
			statusCode: http.StatusNotFound,
			method:     "POST",
			testFunc: func(c *Client, url string) error {
				var result map[string]string
				return c.PostJSONUnmarshal(url, map[string]string{}, &result)
			},
		},
		{
			statusCode: http.StatusForbidden,
			method:     "DELETE",
			testFunc: func(c *Client, url string) error {
				var result map[string]string
				return c.DeleteJSON(url, &result)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_%s", tc.statusCode, tc.method), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte("error message"))
			}))
			defer server.Close()

			client := NewClientWithDefaults()
			err := tc.testFunc(client, server.URL)
			assert.Error(t, err)
		})
	}
}

// TestJSONMarshalError tests error handling for invalid JSON
func TestJSONMarshalError(t *testing.T) {
	client := NewClientWithDefaults()

	// Create a channel which cannot be marshaled to JSON
	invalidData := make(chan bool)
	_, err := client.PostJSON("http://example.com", invalidData)
	require.Error(t, err)
}

// TestContentTypeHeader tests that Content-Type is properly set
func TestContentTypeHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithDefaults()
	_, err := client.PostJSON(server.URL, map[string]string{"test": "data"})
	require.NoError(t, err)
}

// TestTransportConfiguration tests transport settings
func TestTransportConfiguration(t *testing.T) {
	config := Config{
		MaxIdleConns:      25,
		IdleConnTimeout:   60 * time.Second,
		DisableKeepAlives: true,
	}

	client := NewClient(config)
	transport := client.Transport.(*http.Transport)

	assert.Equal(t, 25, transport.MaxIdleConns)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
	assert.True(t, transport.DisableKeepAlives)
}

// TestMultipleRequests tests that client can handle multiple sequential requests
func TestMultipleRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"request": requestCount})
	}))
	defer server.Close()

	client := NewClientWithDefaults()

	for i := 1; i <= 5; i++ {
		var result map[string]int
		err := client.GetJSON(server.URL, &result)
		require.NoError(t, err)
		assert.Equal(t, i, result["request"])
	}
}

// TestInsecureSkipVerify tests TLS configuration without verification
func TestInsecureSkipVerify(t *testing.T) {
	config := Config{
		SkipTLSVerify: true,
	}
	client := NewClient(config)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	tlsConfig := transport.TLSClientConfig
	require.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

// TestSecureSkipVerifyFalse tests that TLS verification is enforced by default
func TestSecureSkipVerifyFalse(t *testing.T) {
	config := Config{
		SkipTLSVerify: false,
	}
	client := NewClient(config)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	// When SkipTLSVerify is false, TLSClientConfig should be nil (use defaults)
	assert.Nil(t, transport.TLSClientConfig)
}

// TestNoTLSConfigWhenNotSkipping tests that no TLS config is created when not skipping verification
func TestNoTLSConfigWhenNotSkipping(t *testing.T) {
	config := Config{}
	client := NewClient(config)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.TLSClientConfig)
}
