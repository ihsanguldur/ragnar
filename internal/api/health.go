package api

import "net/http"

type HealthResponse struct {
	Status string `json:"status"`
}

// Health handles GET /healthz. It only reports that the process is up and
// serving HTTP; it does not check the database or Ollama.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}
