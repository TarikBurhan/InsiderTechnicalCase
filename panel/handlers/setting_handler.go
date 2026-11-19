package handlers

import (
	"log/slog"
	"net/http"

	"notifier/panel/request"
	"notifier/panel/response"
	"notifier/panel/service"
)

// SettingHandler serves /api/v1/settings and /api/v1/system/time.
type SettingHandler struct {
	svc *service.SettingService
	log *slog.Logger
}

// NewSettingHandler creates the handler.
func NewSettingHandler(svc *service.SettingService, log *slog.Logger) *SettingHandler {
	return &SettingHandler{svc: svc, log: log}
}

// List handles GET /api/v1/settings.
func (h *SettingHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	out := make([]response.SettingResponse, 0, len(items))
	for _, s := range items {
		out = append(out, response.FromSetting(s))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// Get handles GET /api/v1/settings/{key}.
func (h *SettingHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context(), r.PathValue("key"))
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.FromSetting(s))
}

// Update handles PUT /api/v1/settings/{key}.
func (h *SettingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req request.UpdateSettingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	s, note, err := h.svc.Update(r.Context(), r.PathValue("key"), req.Value)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.UpdateSettingResponse{SettingResponse: response.FromSetting(s), Note: note})
}

// TimeStatus handles GET /api/v1/system/time.
func (h *SettingHandler) TimeStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.TimeStatus(r.Context())
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.TimeStatusResponse{
		AccountTimezone:         st.AccountTimezone,
		ServiceTimezone:         st.ServiceTimezone,
		ContainerTZ:             st.ContainerTZ,
		DatabaseTimezone:        st.Database.DatabaseTimezone,
		DatabaseSessionTimezone: st.Database.SessionTimezone,
		ServiceNow:              st.ServiceNow,
		DatabaseNow:             st.Database.Now,
		ClockDriftMs:            st.ClockDrift.Milliseconds(),
		Consistent:              len(st.Problems) == 0,
		Problems:                st.Problems,
	})
}
