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

type DocumentResponse struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
	PageCount *int      `json:"page_count,omitempty"`
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
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
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
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}

	sum = hex.EncodeToString(hash.Sum(nil))
	if err := os.Rename(tmp.Name(), filepath.Join(dir, sum+".pdf")); err != nil {
		return "", 0, err
	}
	return sum, size, nil
}
