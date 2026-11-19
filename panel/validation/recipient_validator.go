package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"notifier/panel/dto"
)

var (
	e164Pattern      = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)
	pushTokenPattern = regexp.MustCompile(`^\S{8,1000}$`)
)

// ValidateRecipient validates one address for a channel.
func ValidateRecipient(channel, recipient string) error {
	switch channel {
	case dto.ChannelSMS:
		if !e164Pattern.MatchString(recipient) {
			return fmt.Errorf("%q is not an E.164 phone number", recipient)
		}
	case dto.ChannelEmail:
		addr, err := mail.ParseAddress(recipient)
		if err != nil || addr.Address != recipient {
			return fmt.Errorf("%q is not a valid email address", recipient)
		}
	case dto.ChannelPush:
		if !pushTokenPattern.MatchString(recipient) {
			return fmt.Errorf("%q is not a valid device token", recipient)
		}
	default:
		return fmt.Errorf("unsupported channel %q", channel)
	}
	return nil
}

// NormalizeRecipients trims, validates and de-duplicates DIRECT recipients.
func NormalizeRecipients(channel string, recipients []string, max int) ([]string, error) {
	if len(recipients) == 0 {
		return nil, New("audience.recipients", "at least one recipient is required")
	}
	if len(recipients) > max {
		return nil, New("audience.recipients", fmt.Sprintf("at most %d recipients allowed", max))
	}
	var errs Errors
	seen := make(map[string]struct{}, len(recipients))
	out := make([]string, 0, len(recipients))
	for i, r := range recipients {
		r = strings.TrimSpace(r)
		if channel == dto.ChannelEmail {
			r = strings.ToLower(r)
		}
		if err := ValidateRecipient(channel, r); err != nil {
			errs.Add(fmt.Sprintf("audience.recipients[%d]", i), err.Error())
			continue
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return out, nil
}
