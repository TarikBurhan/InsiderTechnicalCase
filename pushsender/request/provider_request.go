// Package request contains the request sent to the external push provider.
package request

import (
	"encoding/json"

	"notifier/pushsender/dto"
)

// ProviderRequest is the body POSTed to the provider (webhook.site). It
// follows the common {to, channel, content} contract plus push fields.
type ProviderRequest struct {
	To       string          `json:"to"`
	Channel  string          `json:"channel"`
	Title    string          `json:"title,omitempty"`
	Content  string          `json:"content"`
	ImageURL string          `json:"image_url,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// NewProviderRequest builds the push provider request.
func NewProviderRequest(to string, c dto.MessageContent) ProviderRequest {
	return ProviderRequest{To: to, Channel: "push", Title: c.Title, Content: c.Body, ImageURL: c.ImageURL, Data: c.Data}
}
