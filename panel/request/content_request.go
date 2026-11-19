package request

// CreateContentRequest saves a reusable content/template for a channel.
// subject is required for email, title for push.
type CreateContentRequest struct {
	Name     string         `json:"name"`
	Subject  string         `json:"subject,omitempty"`
	Title    string         `json:"title,omitempty"`
	Body     string         `json:"body"`
	IsHTML   *bool          `json:"is_html,omitempty"`
	ImageURL string         `json:"image_url,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}
