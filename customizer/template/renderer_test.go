package template

import (
	"strings"
	"testing"

	"notifier/customizer/dto"
)

func TestRender(t *testing.T) {
	vars := dto.ContactVariables{"first_name": "Ayse", "city": "Izmir", "last_name": ""}
	cases := map[string]string{
		"Hi {{first_name}}!":                "Hi Ayse!",
		"Hi {{ first_name }} from {{CITY}}": "Hi Ayse from Izmir",
		"Dear {{last_name|Customer}}":       "Dear Customer",
		"Dear {{unknown_col|friend}}":       "Dear friend",
		"Nothing here":                      "Nothing here",
		"{{first_name}} {{first_name}}":     "Ayse Ayse",
		"Missing {{missing}} value":         "Missing  value",
	}
	for in, want := range cases {
		if got := Render(in, vars, false); got != want {
			t.Errorf("Render(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderEscapesHTML(t *testing.T) {
	vars := dto.ContactVariables{"first_name": "<script>x</script>"}
	got := Render("<p>{{first_name}}</p>", vars, true)
	if strings.Contains(got, "<script>") {
		t.Fatalf("value was not escaped: %s", got)
	}
}

func TestRenderContent(t *testing.T) {
	c := dto.MessageContent{Subject: "Hi {{first_name}}", Title: "T {{city}}", Body: "B {{first_name}}"}
	out := RenderContent(c, dto.ContactVariables{"first_name": "Can", "city": "Bursa"})
	if out.Subject != "Hi Can" || out.Title != "T Bursa" || out.Body != "B Can" {
		t.Fatalf("unexpected %+v", out)
	}
	if c.Body != "B {{first_name}}" {
		t.Fatal("original content must not be modified")
	}
}

func TestValidateRendered(t *testing.T) {
	if err := ValidateRendered(dto.ChannelSMS, dto.MessageContent{Body: strings.Repeat("a", SMSBodyMaxLength)}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRendered(dto.ChannelSMS, dto.MessageContent{Body: strings.Repeat("a", SMSBodyMaxLength+1)}); err == nil {
		t.Fatal("expected limit error")
	}
	if err := ValidateRendered(dto.ChannelPush, dto.MessageContent{Title: strings.Repeat("a", PushTitleMaxLength+1), Body: "b"}); err == nil {
		t.Fatal("expected push title limit error")
	}
}
