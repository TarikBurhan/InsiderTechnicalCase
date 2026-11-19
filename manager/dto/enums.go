// Package dto contains the Manager's data transfer objects.
package dto

import "strings"

// Channels handled by the system.
const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"
	ChannelPush  = "push"
)

// Channels lists every channel.
var Channels = []string{ChannelSMS, ChannelEmail, ChannelPush}

// Notification statuses.
const (
	StatusNew        = "NEW"
	StatusProcessing = "PROCESSING"
	StatusSending    = "SENDING"
	StatusComplete   = "COMPLETE"
	StatusError      = "ERROR"
	StatusCancelled  = "CANCELLED"
)

// Audience types.
const AudienceTypeDirect = "DIRECT"

// Log levels.
const (
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// AddressKeyExpr is the SQL expression used to detect duplicate addresses
// of a channel (emails are case-insensitive).
func AddressKeyExpr(channel string) string {
	col := "mc." + AddressColumn(channel)
	if channel == ChannelEmail {
		return "lower(btrim(" + col + "))"
	}
	return "btrim(" + col + ")"
}

// AddressKey is the Go equivalent of AddressKeyExpr.
func AddressKey(channel, address string) string {
	address = strings.TrimSpace(address)
	if channel == ChannelEmail {
		return strings.ToLower(address)
	}
	return address
}

// AddressColumn returns the master_contact column used as address for a channel.
func AddressColumn(channel string) string {
	switch channel {
	case ChannelSMS:
		return "phone"
	case ChannelEmail:
		return "email"
	default:
		return "device_token"
	}
}
