// Package template personalises message content by substituting
// {{column}} / {{column|default}} variables with master_contact values.
package template

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"notifier/customizer/dto"
)

var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|([^{}]*))?\}\}`)

// Render replaces every variable in tpl. Missing / empty values fall back to
// the inline default ({{first_name|Customer}}) or an empty string. When
// escapeHTML is set values are HTML-escaped (HTML emails).
func Render(tpl string, vars dto.ContactVariables, escapeHTML bool) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	return variablePattern.ReplaceAllStringFunc(tpl, func(match string) string {
		sub := variablePattern.FindStringSubmatch(match)
		value := vars[strings.ToLower(sub[1])]
		if value == "" {
			value = strings.TrimSpace(sub[2])
		}
		if escapeHTML {
			return html.EscapeString(value)
		}
		return value
	})
}

// RenderContent personalises every text field of a content.
func RenderContent(c dto.MessageContent, vars dto.ContactVariables) dto.MessageContent {
	out := c
	out.Subject = Render(c.Subject, vars, false)
	out.Title = Render(c.Title, vars, false)
	out.Body = Render(c.Body, vars, c.IsHTML)
	return out
}

// Channel limits re-checked after rendering (values may make text longer).
const (
	SMSBodyMaxLength      = 480
	EmailSubjectMaxLength = 255
	PushTitleMaxLength    = 100
	PushBodyMaxLength     = 500
)

// ValidateRendered checks the personalised content against channel limits.
func ValidateRendered(channel string, c dto.MessageContent) error {
	check := func(field, value string, max int) error {
		if n := utf8.RuneCountInString(value); n > max {
			return fmt.Errorf("rendered %s is %d characters, limit is %d", field, n, max)
		}
		return nil
	}
	switch channel {
	case dto.ChannelSMS:
		return check("body", c.Body, SMSBodyMaxLength)
	case dto.ChannelEmail:
		return check("subject", c.Subject, EmailSubjectMaxLength)
	case dto.ChannelPush:
		if err := check("title", c.Title, PushTitleMaxLength); err != nil {
			return err
		}
		return check("body", c.Body, PushBodyMaxLength)
	}
	return nil
}
