package queries

import (
	"fmt"

	"notifier/panel/dto"
)

// Each content table is projected onto the same column list so a single
// scanner can read all of them:
// id, name, subject, title, body, is_html, image_url, data, variables, created_at, updated_at
const (
	smsContentProjection   = `id, name, NULL::text, NULL::text, body, false, NULL::text, NULL::jsonb, variables, created_at, updated_at`
	emailContentProjection = `id, name, subject, NULL::text, body, is_html, NULL::text, NULL::jsonb, variables, created_at, updated_at`
	pushContentProjection  = `id, name, NULL::text, title, body, false, image_url, data, variables, created_at, updated_at`
)

// InsertSMSContent saves an SMS content.
const InsertSMSContent = `INSERT INTO sms_content (name, body, variables) VALUES ($1, $2, $3) RETURNING ` + smsContentProjection

// InsertEmailContent saves an email content.
const InsertEmailContent = `INSERT INTO email_content (name, subject, body, is_html, variables) VALUES ($1, $2, $3, $4, $5) RETURNING ` + emailContentProjection

// InsertPushContent saves a push content.
const InsertPushContent = `INSERT INTO push_content (name, title, body, image_url, data, variables) VALUES ($1, $2, $3, $4, $5, $6) RETURNING ` + pushContentProjection

func contentTable(channel string) (table, projection string, err error) {
	switch channel {
	case dto.ChannelSMS:
		return "sms_content", smsContentProjection, nil
	case dto.ChannelEmail:
		return "email_content", emailContentProjection, nil
	case dto.ChannelPush:
		return "push_content", pushContentProjection, nil
	}
	return "", "", fmt.Errorf("unknown channel %q", channel)
}

// GetContent returns the select-by-id query of a channel content table.
func GetContent(channel string) (string, error) {
	table, proj, err := contentTable(channel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`SELECT %s FROM %s WHERE id = $1`, proj, table), nil
}

// ListContents returns the paginated list query of a channel content table.
func ListContents(channel string) (string, error) {
	table, proj, err := contentTable(channel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`SELECT %s FROM %s ORDER BY id DESC LIMIT $1 OFFSET $2`, proj, table), nil
}

// CountContents returns the count query of a channel content table.
func CountContents(channel string) (string, error) {
	table, _, err := contentTable(channel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`SELECT count(*) FROM %s`, table), nil
}

// DeleteContent returns the delete query of a channel content table.
func DeleteContent(channel string) (string, error) {
	table, _, err := contentTable(channel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, table), nil
}
