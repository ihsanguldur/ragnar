package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes v as the response body. Headers must be set before
// WriteHeader, and the status cannot change once the body has started, so an
// encoding error here can only be dropped.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ErrorResponse is the body of every error reply, and the payload of the
// error event on a stream. Messages are short and safe to show to clients;
// details go to the server log instead.
type ErrorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
