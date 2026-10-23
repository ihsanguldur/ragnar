package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ihsanguldur/ragnar/internal/store"
)

// DocumentResponse describes an uploaded document. Status moves from pending
// to processing to ready, or to failed with Error set. PageCount is set once
// the document is ready.
type DocumentResponse struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
	PageCount *int      `json:"page_count,omitempty"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newDocumentResponse(d store.Document) DocumentResponse {
	return DocumentResponse{
		ID:        d.ID,
		Filename:  d.Filename,
		SHA256:    d.SHA256,
		SizeBytes: d.SizeBytes,
		Status:    d.Status,
		Error:     d.Error,
		PageCount: d.PageCount,
		Attempts:  d.Attempts,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// UploadDocument handles POST /documents with a multipart "file" field. It
// stores the PDF and queues it for ingestion. The response is 202 Accepted
// for a new document, and 200 OK with the existing record when the same file
// was uploaded before.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// <<20 converts megabytes to bytes.
	r.Body = http.MaxBytesReader(w, r.Body, h.ingest.MaxUploadMB<<20)

	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file is larger than %d MB", h.ingest.MaxUploadMB))
			return
		}
		writeError(w, http.StatusBadRequest, `multipart form with a "file" field is required`)
		return
	}
	defer file.Close()

	// Every PDF starts with "%PDF-". Checking the content rather than the
	// file extension or the client's Content-Type rejects other files that
	// were merely renamed. The file is then rewound so it is saved in full.
	magic := make([]byte, 5)
	if _, err := io.ReadFull(file, magic); err != nil || string(magic) != "%PDF-" {
		writeError(w, http.StatusBadRequest, "file is not a PDF")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		slog.Error("upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not read file")
		return
	}

	sum, size, err := saveUpload(h.ingest.UploadDir, file)
	if err != nil {
		slog.Error("upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not save file")
		return
	}

	doc, created, err := h.store.CreateDocument(r.Context(), header.Filename, sum, size)
	if err != nil {
		slog.Error("upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not save document")
		return
	}

	slog.Info("document uploaded", "id", doc.ID, "filename", header.Filename, "size_bytes", size, "created", created)

	status := http.StatusAccepted
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, newDocumentResponse(doc))
}

// saveUpload writes src to dir/<sha256>.pdf and returns the hash and size.
// The hash is computed while the file is written, in a single pass. Naming
// files by content means identical uploads share one file. Writing to a
// temporary file and renaming it at the end means a half-written file never
// appears under the final name; the deferred Remove cleans up after a
// failure and does nothing once the rename has succeeded.
func saveUpload(dir string, src io.Reader) (sum string, size int64, err error) {
	tmp, err := os.CreateTemp(dir, "upload-*.tmp")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hash := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, hash), src)
	if err != nil {
		return "", 0, err
	}
	// Close reports write errors that Copy may not have seen, so it is
	// checked before the file is renamed.
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}

	sum = hex.EncodeToString(hash.Sum(nil))
	if err := os.Rename(tmp.Name(), filepath.Join(dir, sum+".pdf")); err != nil {
		return "", 0, err
	}
	return sum, size, nil
}

// ListDocuments handles GET /documents.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := h.store.ListDocuments(r.Context())
	if err != nil {
		slog.Error("list documents failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not list documents")
		return
	}

	resp := make([]DocumentResponse, 0, len(docs))
	for _, d := range docs {
		resp = append(resp, newDocumentResponse(d))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetDocument handles GET /documents/{id}. Clients poll it after an upload
// to see when the document is ready.
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	doc, found, err := h.store.GetDocument(r.Context(), r.PathValue("id"))
	if err != nil {
		slog.Error("get document failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not get document")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, newDocumentResponse(doc))
}
