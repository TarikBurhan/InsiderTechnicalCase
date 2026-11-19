// Package request contains the request sent to the external email provider.
package request

import "notifier/emailsender/dto"

// ProviderRequest is the body POSTed to the provider (webhook.site). It
// follows the common {to, channel, content} contract plus email fields.
type ProviderRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Subject string `json:"subject,omitempty"`
	Content string `json:"content"`
	IsHTML  bool   `json:"is_html,omitempty"`
}

// NewProviderRequest builds the email provider request.
func NewProviderRequest(to string, c dto.MessageContent) ProviderRequest {
	return ProviderRequest{To: to, Channel: "email", Subject: c.Subject, Content: c.Body, IsHTML: c.IsHTML}
}
