package main

// Request represents an incoming API request.
type Request struct {
	ID      string `json:"id"`
	Payload string `json:"payload"`
}

// Response represents the API response.
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
