// Package response contains the provider's response body.
package response

// ProviderResponse is the expected 202 body of the provider.
type ProviderResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}
