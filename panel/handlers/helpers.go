// Package handlers contains the Panel's HTTP handlers. They only translate
// HTTP <-> service calls; business rules live in package service.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"notifier/panel/response"
	"notifier/panel/service"
	"notifier/panel/validation"
	"notifier/pkg/correlation"
)

const maxBodyBytes = 10 << 20 // 10 MiB (batch of 1000 notifications)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details []response.FieldError) {
	writeJSON(w, status, response.ErrorResponse{
		Error: code, Message: message, Details: details, CorrelationID: correlation.FromContext(r.Context()),
	})
}

// decodeJSON strictly decodes the request body into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", fmt.Sprintf("invalid request body: %v", err), nil)
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object", nil)
		return false
	}
	return true
}

// handleServiceError maps service errors to HTTP responses.
func handleServiceError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var verrs validation.Errors
	switch {
	case errors.As(err, &verrs):
		details := make([]response.FieldError, len(verrs))
		for i, fe := range verrs {
			details[i] = response.FieldError{Field: fe.Field, Message: fe.Message}
		}
		writeError(w, r, http.StatusUnprocessableEntity, "validation_failed", "request validation failed", details)
	case errors.Is(err, service.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found", nil)
	case errors.Is(err, service.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", err.Error(), nil)
	default:
		log.ErrorContext(r.Context(), "request failed", slog.String("path", r.URL.Path), slog.Any("error", err))
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", nil)
	}
}

// pathUUID reads and validates a UUID path value.
func pathUUID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	v := r.PathValue(name)
	if _, err := uuid.Parse(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_id", name+" must be a valid UUID", nil)
		return "", false
	}
	return v, true
}

// pathInt64 reads and validates an integer path value.
func pathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || v <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", name+" must be a positive integer", nil)
		return 0, false
	}
	return v, true
}

// Paging holds pagination settings.
type Paging struct {
	Default int
	Max     int
}

// parse reads page/page_size query parameters.
func (p Paging) parse(r *http.Request) (page, size, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	size, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = p.Default
	}
	if size > p.Max {
		size = p.Max
	}
	return page, size, (page - 1) * size
}
