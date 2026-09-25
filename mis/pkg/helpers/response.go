package helpers

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a structured error payload.
type ErrorResponse struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

// WriteError writes a JSON-encoded error response with the given status code.
func WriteError(w http.ResponseWriter, status int, message, detail string) {
	WriteJSON(w, status, ErrorResponse{
		Error:  message,
		Detail: detail,
	})
}

// WriteJSON serializes v to JSON and writes it to the response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}
