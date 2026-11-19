package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"notifier/panel/dto"
)

// Channel content limits.
const (
	SMSBodyMaxLength      = 480 // 3 concatenated SMS segments
	EmailSubjectMaxLength = 255
	EmailBodyMaxLength    = 100_000
	PushTitleMaxLength    = 100
	PushBodyMaxLength     = 500
	PushDataMaxBytes      = 4096
	ContentNameMaxLength  = 100
)

// VariablePattern matches template variables like {{first_name}} or
// {{first_name|Customer}} (with a default value).
var VariablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|([^{}]*))?\}\}`)

// ExtractVariables returns the sorted unique variable names used in texts.
func ExtractVariables(texts ...string) []string {
	seen := map[string]struct{}{}
	for _, t := range texts {
		for _, m := range VariablePattern.FindAllStringSubmatch(t, -1) {
			seen[strings.ToLower(m[1])] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ValidateContent checks required fields, limits and placeholder syntax.
func ValidateContent(channel string, c dto.MessageContent, prefix string) Errors {
	var errs Errors
	switch channel {
	case dto.ChannelSMS:
		requireText(&errs, prefix+"body", c.Body, SMSBodyMaxLength)
	case dto.ChannelEmail:
		requireText(&errs, prefix+"subject", c.Subject, EmailSubjectMaxLength)
		requireText(&errs, prefix+"body", c.Body, EmailBodyMaxLength)
	case dto.ChannelPush:
		requireText(&errs, prefix+"title", c.Title, PushTitleMaxLength)
		requireText(&errs, prefix+"body", c.Body, PushBodyMaxLength)
		if c.ImageURL != "" {
			if u, err := url.ParseRequestURI(c.ImageURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				errs.Add(prefix+"image_url", "must be a valid http(s) url")
			}
		}
		if len(c.Data) > PushDataMaxBytes {
			errs.Add(prefix+"data", fmt.Sprintf("must be at most %d bytes", PushDataMaxBytes))
		}
	default:
		errs.Add("channel", "unsupported channel")
	}
	for field, text := range map[string]string{"body": c.Body, "subject": c.Subject, "title": c.Title} {
		if strings.Count(text, "{{") != len(VariablePattern.FindAllString(text, -1)) {
			errs.Add(prefix+field, "contains a malformed {{variable}} placeholder")
		}
	}
	return errs
}

// ValidateVariables checks that every variable is a master_contact column.
func ValidateVariables(vars []string, allowedColumns map[string]string, field string) Errors {
	var errs Errors
	for _, v := range vars {
		if _, ok := allowedColumns[v]; !ok {
			errs.Add(field, fmt.Sprintf("unknown variable {{%s}}: not a master_contact column", v))
		}
	}
	return errs
}

func requireText(errs *Errors, field, value string, max int) {
	if strings.TrimSpace(value) == "" {
		errs.Add(field, "is required")
		return
	}
	if n := utf8.RuneCountInString(value); n > max {
		errs.Add(field, fmt.Sprintf("must be at most %d characters (got %d)", max, n))
	}
}
