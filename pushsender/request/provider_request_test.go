package request

import (
	"encoding/json"
	"testing"

	"notifier/pushsender/dto"
)

func TestNewProviderRequest(t *testing.T) {
	r := NewProviderRequest("token-123456", dto.MessageContent{Title: "T", Body: "B", Data: json.RawMessage(`{"k":"v"}`)})
	if r.To != "token-123456" || r.Channel != "push" || r.Title != "T" || r.Content != "B" || string(r.Data) != `{"k":"v"}` {
		t.Fatalf("unexpected request %+v", r)
	}
}
