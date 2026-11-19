package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"notifier/panel/dto"
	"notifier/panel/request"
	"notifier/panel/response"
	"notifier/panel/service"
)

// IdempotencyHeader may carry the idempotency key instead of the body.
const IdempotencyHeader = "Idempotency-Key"

// NotificationHandler serves /api/v1/notifications.
type NotificationHandler struct {
	svc    *service.NotificationService
	paging Paging
	log    *slog.Logger
}

// NewNotificationHandler creates the handler.
func NewNotificationHandler(svc *service.NotificationService, paging Paging, log *slog.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, paging: paging, log: log}
}

func toCreateResponse(res service.CreateResult) response.CreateNotificationResponse {
	return response.CreateNotificationResponse{
		NotificationResponse: response.FromNotification(res.Notification, nil),
		Duplicate:            !res.Created,
		AudienceCount:        res.AudienceCount,
	}
}

// Create handles POST /api/v1/notifications.
func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateNotificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = r.Header.Get(IdempotencyHeader)
	}
	res, err := h.svc.Create(r.Context(), req)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	status := http.StatusCreated
	if !res.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, toCreateResponse(res))
}

// CreateBatch handles POST /api/v1/notifications/batch.
func (h *NotificationHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var req request.BatchCreateNotificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	batchID, results, err := h.svc.CreateBatch(r.Context(), req)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	out := response.BatchCreateResponse{BatchID: batchID, Count: len(results)}
	for _, res := range results {
		out.Notifications = append(out.Notifications, toCreateResponse(res))
	}
	writeJSON(w, http.StatusCreated, out)
}

// Get handles GET /api/v1/notifications/{id}.
func (h *NotificationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	n, stats, err := h.svc.Get(r.Context(), id)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.FromNotification(n, &stats))
}

// GetBatch handles GET /api/v1/batches/{batchId}.
func (h *NotificationHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	batchID, ok := pathUUID(w, r, "batchId")
	if !ok {
		return
	}
	items, err := h.svc.GetBatch(r.Context(), batchID)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	out := response.BatchStatusResponse{BatchID: batchID, Total: len(items), StatusCounts: map[string]int{}}
	for _, n := range items {
		out.StatusCounts[n.Status]++
		out.Notifications = append(out.Notifications, response.FromNotification(n, nil))
	}
	writeJSON(w, http.StatusOK, out)
}

// List handles GET /api/v1/notifications.
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, size, offset := h.paging.parse(r)
	f := dto.NotificationFilter{
		Status:   strings.ToUpper(q.Get("status")),
		Channel:  strings.ToLower(q.Get("channel")),
		Priority: strings.ToLower(q.Get("priority")),
		BatchID:  q.Get("batch_id"),
		Limit:    size,
		Offset:   offset,
	}
	if f.Status != "" && !dto.IsValidStatus(f.Status) {
		writeError(w, r, http.StatusBadRequest, "invalid_filter", "unknown status", nil)
		return
	}
	if f.Channel != "" && !dto.IsValidChannel(f.Channel) {
		writeError(w, r, http.StatusBadRequest, "invalid_filter", "unknown channel", nil)
		return
	}
	if f.Priority != "" && !dto.IsValidPriority(f.Priority) {
		writeError(w, r, http.StatusBadRequest, "invalid_filter", "unknown priority", nil)
		return
	}
	if f.BatchID != "" {
		if _, err := uuid.Parse(f.BatchID); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_filter", "batch_id must be a UUID", nil)
			return
		}
	}
	for key, target := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if v := q.Get(key); v != "" {
			t, err := request.ParseFlexibleTime(v)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_filter", key+" must be RFC3339 or a local date-time (account timezone)", nil)
				return
			}
			*target = &t
		}
	}

	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	data := make([]response.NotificationResponse, 0, len(items))
	for _, n := range items {
		data = append(data, response.FromNotification(n, nil))
	}
	writeJSON(w, http.StatusOK, response.ListResponse[response.NotificationResponse]{
		Data: data, Pagination: response.NewPagination(page, size, total),
	})
}

// Cancel handles POST /api/v1/notifications/{id}/cancel.
func (h *NotificationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	n, err := h.svc.Cancel(r.Context(), id)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, response.FromNotification(n, nil))
}

// Deliveries handles GET /api/v1/notifications/{id}/deliveries.
func (h *NotificationHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	page, size, offset := h.paging.parse(r)
	items, total, err := h.svc.Deliveries(r.Context(), id, r.URL.Query().Get("status"), size, offset)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	data := make([]response.DeliveryResponse, 0, len(items))
	for _, d := range items {
		data = append(data, response.FromDelivery(d))
	}
	writeJSON(w, http.StatusOK, response.ListResponse[response.DeliveryResponse]{
		Data: data, Pagination: response.NewPagination(page, size, total),
	})
}

// Logs handles GET /api/v1/notifications/{id}/logs.
func (h *NotificationHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	page, size, offset := h.paging.parse(r)
	items, total, err := h.svc.Logs(r.Context(), id, size, offset)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}
	data := make([]response.LogResponse, 0, len(items))
	for _, l := range items {
		data = append(data, response.FromLog(l))
	}
	writeJSON(w, http.StatusOK, response.ListResponse[response.LogResponse]{
		Data: data, Pagination: response.NewPagination(page, size, total),
	})
}
