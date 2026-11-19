package request

import (
	"testing"

	"notifier/emailsender/dto"
)

func TestNewProviderRequest(t *testing.T) {
	r := NewProviderRequest("a@b.io", dto.MessageContent{Subject: "S", Body: "<p>B</p>", IsHTML: true})
	if r.To != "a@b.io" || r.Channel != "email" || r.Subject != "S" || r.Content != "<p>B</p>" || !r.IsHTML {
		t.Fatalf("unexpected request %+v", r)
	}
}
