package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"notifier/panel/request"
	"notifier/panel/response"
	"notifier/panel/service"
)

// ContentHandler serves /api/v1/contents/{channel}.
type ContentHandler struct {
	svc    *service.ContentService
	paging Paging
	log    *slog.Logger
}

// NewContentHandler creates the handler.
func NewContentHandler(svc *service.ContentService, paging Paging, log *slog.Logger) *ContentHandler {
	return &ContentHandler{svc: svc, paging: paging, log: log}
}

func channelParam(r *http.Request) string { return strings.ToLower(r.PathValue("channel")) }

// Create handles POST /api/v1/contents/{channel}.
func (h *ContentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateContentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := h.svc.Create(r.Context(), channelParam(r), req)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, response.FromContent(c))
}

// Get handles GET /api/v1/contents/{channel}/{id}.
func (h *ContentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), channelParam(r), id)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.FromContent(c))
}

// List handles GET /api/v1/contents/{channel}.
func (h *ContentHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size, offset := h.paging.parse(r)
	items, total, err := h.svc.List(r.Context(), channelParam(r), size, offset)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	data := make([]response.ContentResponse, 0, len(items))
	for _, c := range items {
		data = append(data, response.FromContent(c))
	}
	writeJSON(w, http.StatusOK, response.ListResponse[response.ContentResponse]{
		Data: data, Pagination: response.NewPagination(page, size, total),
	})
}

// Delete handles DELETE /api/v1/contents/{channel}/{id}.
func (h *ContentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), channelParam(r), id); err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
