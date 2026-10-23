package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter maps the HTTP routes to h's methods. Every request gets an ID, an
// access log line, and panic recovery, so one failing request cannot crash
// the server.
func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.Health)
	// Uploads are processed in the background; poll GET /documents/{id}
	// until its status is ready.
	r.Post("/documents", h.UploadDocument)
	r.Get("/documents", h.ListDocuments)
	r.Get("/documents/{id}", h.GetDocument)
	// /search returns the raw retrieved chunks without calling the chat
	// model; it is useful for checking what /ask would see.
	r.Get("/search", h.Search)
	r.Post("/ask", h.Ask)

	return r
}
