package dto

import (
	"encoding/json"
	"time"
)

// Content is a stored, pre-validated content/template of any channel.
type Content struct {
	ID        int64
	Channel   string
	Name      string
	Subject   *string
	Title     *string
	Body      string
	IsHTML    bool
	ImageURL  *string
	Data      json.RawMessage
	Variables []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ToMessageContent converts a stored content into a notification snapshot.
func (c Content) ToMessageContent() MessageContent {
	mc := MessageContent{Body: c.Body, IsHTML: c.IsHTML, Data: c.Data}
	if c.Subject != nil {
		mc.Subject = *c.Subject
	}
	if c.Title != nil {
		mc.Title = *c.Title
	}
	if c.ImageURL != nil {
		mc.ImageURL = *c.ImageURL
	}
	return mc
}
