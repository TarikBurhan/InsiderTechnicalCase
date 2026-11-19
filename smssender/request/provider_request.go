// Package request contains the request sent to the external SMS provider.
package request

import "notifier/smssender/dto"

// ProviderRequest is the body POSTed to the provider (webhook.site).
type ProviderRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

// NewProviderRequest builds the SMS provider request.
func NewProviderRequest(to string, c dto.MessageContent) ProviderRequest {
	return ProviderRequest{To: to, Channel: "sms", Content: c.Body}
}
