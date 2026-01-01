package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HandleRequest processes the incoming HTTP request.
func HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process the request (simulate work)
	fmt.Printf("Processing request ID: %s\n", req.ID)

	resp := Response{
		Status:  "success",
		Message: fmt.Sprintf("Processed payload: %s", req.Payload),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
