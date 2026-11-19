package validation

import (
	"reflect"
	"strings"
	"testing"

	"notifier/panel/dto"
)

func TestExtractVariables(t *testing.T) {
	got := ExtractVariables("Hi {{first_name}} from {{ city }}", "{{first_name|Friend}} {{Loyalty_Tier}}")
	want := []string{"city", "first_name", "loyalty_tier"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		channel string
		content dto.MessageContent
		wantErr bool
	}{
		{"sms ok", dto.ChannelSMS, dto.MessageContent{Body: "hello"}, false},
		{"sms empty", dto.ChannelSMS, dto.MessageContent{Body: "  "}, true},
		{"sms too long", dto.ChannelSMS, dto.MessageContent{Body: strings.Repeat("a", SMSBodyMaxLength+1)}, true},
		{"sms unicode within limit", dto.ChannelSMS, dto.MessageContent{Body: strings.Repeat("ş", SMSBodyMaxLength)}, false},
		{"email ok", dto.ChannelEmail, dto.MessageContent{Subject: "s", Body: "<p>b</p>"}, false},
		{"email no subject", dto.ChannelEmail, dto.MessageContent{Body: "b"}, true},
		{"push ok", dto.ChannelPush, dto.MessageContent{Title: "t", Body: "b", ImageURL: "https://x.io/a.png"}, false},
		{"push bad image", dto.ChannelPush, dto.MessageContent{Title: "t", Body: "b", ImageURL: "ftp://x"}, true},
		{"push no title", dto.ChannelPush, dto.MessageContent{Body: "b"}, true},
		{"malformed placeholder", dto.ChannelSMS, dto.MessageContent{Body: "hi {{first name}}"}, true},
		{"unknown channel", "fax", dto.MessageContent{Body: "x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateContent(tt.channel, tt.content, "content.")
			if (len(errs) > 0) != tt.wantErr {
				t.Fatalf("errs=%v wantErr=%v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateVariables(t *testing.T) {
	cols := map[string]string{"first_name": "text", "city": "text"}
	if errs := ValidateVariables([]string{"first_name", "city"}, cols, "content"); len(errs) != 0 {
		t.Fatalf("unexpected errors %v", errs)
	}
	if errs := ValidateVariables([]string{"password"}, cols, "content"); len(errs) != 1 {
		t.Fatalf("expected one error, got %v", errs)
	}
}

func TestNormalizeRecipients(t *testing.T) {
	got, err := NormalizeRecipients(dto.ChannelSMS, []string{"+905551234567", " +905551234567 ", "+905559999999"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected dedupe to 2, got %v", got)
	}
	if _, err := NormalizeRecipients(dto.ChannelSMS, []string{"05551234567"}, 10); err == nil {
		t.Fatal("expected invalid phone error")
	}
	if _, err := NormalizeRecipients(dto.ChannelEmail, []string{"not-an-email"}, 10); err == nil {
		t.Fatal("expected invalid email error")
	}
	if _, err := NormalizeRecipients(dto.ChannelPush, []string{"a", "b"}, 1); err == nil {
		t.Fatal("expected max recipients error")
	}
	if got, err := NormalizeRecipients(dto.ChannelEmail, []string{"User@Example.com"}, 10); err != nil || got[0] != "user@example.com" {
		t.Fatalf("email should be lower-cased, got %v %v", got, err)
	}
}
