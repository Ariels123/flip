// Package httpclient - Examples demonstrating usage of the HTTP client
//
// This file shows common usage patterns for the httpclient package.
// These examples are not executed as part of tests but serve as documentation.

package httpclient

import (
	"fmt"
	"time"
)

// Example_basicGet demonstrates a simple GET request
func Example_basicGet() {
	client := NewClientWithDefaults()
	resp, err := client.Get("https://api.example.com/data")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Status:", resp.StatusCode)
}

// Example_getJSON demonstrates GET request with automatic JSON unmarshaling
func Example_getJSON() {
	client := NewClientWithDefaults()

	var data struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	err := client.GetJSON("https://api.example.com/user/123", &data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("User: %s (%s)\n", data.Name, data.ID)
}

// Example_postJSON demonstrates POST request with JSON body and response
func Example_postJSON() {
	client := NewClientWithDefaults()

	request := map[string]string{
		"name":  "John",
		"email": "john@example.com",
	}

	var response map[string]interface{}
	err := client.PostJSONUnmarshal("https://api.example.com/users", request, &response)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Created user: %v\n", response)
}

// Example_skipTLS demonstrates creating a client that skips TLS verification
// Useful for self-signed certificates in development
func Example_skipTLS() {
	client := NewClientSkipTLS(30 * time.Second)

	var data interface{}
	err := client.GetJSON("https://self-signed.example.com/api/data", &data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Success:", data)
}

// Example_customConfig demonstrates creating a client with custom configuration
func Example_customConfig() {
	config := Config{
		Timeout:           45 * time.Second,
		MaxRetries:        3,
		SkipTLSVerify:     true,
		MaxIdleConns:      20,
		IdleConnTimeout:   60 * time.Second,
		DisableKeepAlives: false,
	}

	client := NewClient(config)
	resp, err := client.Get("https://api.example.com/endpoint")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Status:", resp.StatusCode)
}

// Example_patchJSON demonstrates PATCH request with JSON body
func Example_patchJSON() {
	client := NewClientWithDefaults()

	updateData := map[string]string{
		"status": "active",
		"role":   "admin",
	}

	var response map[string]interface{}
	err := client.PatchJSONUnmarshal("https://api.example.com/users/123", updateData, &response)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Updated user: %v\n", response)
}

// Example_deleteResource demonstrates DELETE request
func Example_deleteResource() {
	client := NewClientWithDefaults()

	resp, err := client.Delete("https://api.example.com/users/123")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Resource deleted successfully")
}

// Example_retryLogic demonstrates automatic retry behavior
// The client automatically retries on 5xx errors up to MaxRetries times
func Example_retryLogic() {
	config := Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3, // Will retry up to 3 times
	}

	client := NewClient(config)
	// If the server returns 500, 502, 503, etc., the client will retry
	resp, err := client.Get("https://api.example.com/flaky-endpoint")
	if err != nil {
		fmt.Println("Error after retries:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Success:", resp.StatusCode)
}
