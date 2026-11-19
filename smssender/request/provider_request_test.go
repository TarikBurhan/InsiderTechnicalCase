package request

import (
	"testing"

	"notifier/smssender/dto"
)

func TestNewProviderRequest(t *testing.T) {
	r := NewProviderRequest("+905551234567", dto.MessageContent{Body: "Your message"})
	if r.To != "+905551234567" || r.Channel != "sms" || r.Content != "Your message" {
		t.Fatalf("unexpected request %+v", r)
	}
}
