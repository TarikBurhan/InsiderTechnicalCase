package handlers

import (
	"log/slog"
	"net/http"

	"notifier/panel/request"
	"notifier/panel/response"
	"notifier/panel/service"
)

// AudienceHandler serves audience validation and audience groups.
type AudienceHandler struct {
	svc    *service.AudienceService
	paging Paging
	log    *slog.Logger
}

// NewAudienceHandler creates the handler.
func NewAudienceHandler(svc *service.AudienceService, paging Paging, log *slog.Logger) *AudienceHandler {
	return &AudienceHandler{svc: svc, paging: paging, log: log}
}

// Validate handles POST /api/v1/audiences/validate.
func (h *AudienceHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req request.ValidateAudienceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Validate(r.Context(), req)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.AudienceValidationResponse{Valid: true, Type: res.Type, Count: res.Count, ResolvedSQL: res.SQL})
}

// CreateGroup handles POST /api/v1/audience-groups.
func (h *AudienceHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req request.CreateAudienceGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	g, err := h.svc.CreateGroup(r.Context(), req)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, response.FromAudienceGroup(g))
}

// GetGroup handles GET /api/v1/audience-groups/{id}.
func (h *AudienceHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	g, err := h.svc.GetGroup(r.Context(), id)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.FromAudienceGroup(g))
}

// ListGroups handles GET /api/v1/audience-groups.
func (h *AudienceHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	page, size, offset := h.paging.parse(r)
	items, total, err := h.svc.ListGroups(r.Context(), size, offset)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	data := make([]response.AudienceGroupResponse, 0, len(items))
	for _, g := range items {
		data = append(data, response.FromAudienceGroup(g))
	}
	writeJSON(w, http.StatusOK, response.ListResponse[response.AudienceGroupResponse]{
		Data: data, Pagination: response.NewPagination(page, size, total),
	})
}

// DeleteGroup handles DELETE /api/v1/audience-groups/{id}.
func (h *AudienceHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteGroup(r.Context(), id); err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
