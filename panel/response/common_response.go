// Package response contains the JSON bodies returned by the Panel API and
// the mappers that build them from DTOs.
package response

// ErrorResponse is returned for every non 2xx answer.
type ErrorResponse struct {
	Error         string       `json:"error"`
	Message       string       `json:"message"`
	Details       []FieldError `json:"details,omitempty"`
	CorrelationID string       `json:"correlation_id,omitempty"`
}

// FieldError describes a validation problem of a single field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Pagination describes a page of results.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// ListResponse wraps a page of items.
type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// NewPagination computes total pages.
func NewPagination(page, pageSize int, total int64) Pagination {
	pages := int64(0)
	if pageSize > 0 {
		pages = (total + int64(pageSize) - 1) / int64(pageSize)
	}
	return Pagination{Page: page, PageSize: pageSize, Total: total, TotalPages: pages}
}
