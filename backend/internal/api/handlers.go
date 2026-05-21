package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/waiyanphyoaung/email-insights/internal/parser"
	"github.com/waiyanphyoaung/email-insights/internal/service"
	"github.com/waiyanphyoaung/email-insights/internal/store"
)

type Handler struct {
	processor *service.Processor
	store     *store.Store
}

func NewHandler(processor *service.Processor, st *store.Store) *Handler {
	return &Handler{processor: processor, store: st}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Health(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) UploadEmails(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	emails, err := parser.ParseUpload(file, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(emails) == 0 {
		writeError(w, http.StatusBadRequest, "no valid emails in upload")
		return
	}

	result, err := h.processor.ProcessUpload(r.Context(), emails)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UploadEmailsJSON(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	emails, err := parser.ParseUpload(strings.NewReader(string(body)), "upload.json")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.processor.ProcessUpload(r.Context(), emails)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListSpending(w http.ResponseWriter, r *http.Request) {
	expenses, err := h.store.ListExpenses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, expenses)
}

func (h *Handler) SpendingSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.store.SpendingSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) ListSaaS(w http.ResponseWriter, r *http.Request) {
	subs, err := h.store.ListSubscriptions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) SaaSSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.store.SaaSSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) GetEmail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid email ID")
		return
	}
	email, err := h.store.GetEmail(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if email == nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}
	writeJSON(w, http.StatusOK, email)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
