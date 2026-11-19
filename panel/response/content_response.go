package response

import (
	"encoding/json"
	"time"

	"notifier/panel/dto"
)

// ContentResponse is a saved content/template.
type ContentResponse struct {
	ID        int64           `json:"id"`
	Channel   string          `json:"channel"`
	Name      string          `json:"name"`
	Subject   *string         `json:"subject,omitempty"`
	Title     *string         `json:"title,omitempty"`
	Body      string          `json:"body"`
	IsHTML    bool            `json:"is_html,omitempty"`
	ImageURL  *string         `json:"image_url,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Variables []string        `json:"variables"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// FromContent maps a DTO.
func FromContent(c dto.Content) ContentResponse {
	vars := c.Variables
	if vars == nil {
		vars = []string{}
	}
	return ContentResponse{
		ID: c.ID, Channel: c.Channel, Name: c.Name, Subject: c.Subject, Title: c.Title, Body: c.Body,
		IsHTML: c.IsHTML, ImageURL: c.ImageURL, Data: c.Data, Variables: vars,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}
