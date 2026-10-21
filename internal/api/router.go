package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.Health)
	r.Post("/chat", h.Chat)
	r.Post("/documents", h.UploadDocument)
	r.Get("/documents", h.ListDocuments)
	r.Get("/documents/{id}", h.GetDocument)
	r.Get("/search", h.Search)
	r.Post("/ask", h.Ask)

	return r
}
