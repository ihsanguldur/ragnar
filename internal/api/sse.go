package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// startSSE begins a Server-Sent Events response: a long-lived 200 response
// in which the server writes events as they happen. After this, errors can
// no longer change the status code and must be sent as events.
func startSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
}

// writeSSE sends one event, encoded as an "event:" line and a "data:" line of
// JSON followed by a blank line. It flushes right away, because net/http
// buffers writes and the client would otherwise receive tokens in bursts.
// An error usually means the client has disconnected.
func writeSSE(w http.ResponseWriter, event string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}
