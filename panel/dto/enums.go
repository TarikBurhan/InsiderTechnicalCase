// Package dto contains the Panel's internal data transfer objects that map
// database rows and are passed between repository, service and handler.
package dto

// Notification channels.
const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"
	ChannelPush  = "push"
)

// Notification priorities.
const (
	PriorityHigh   = "high"
	PriorityNormal = "normal"
	PriorityLow    = "low"
)

// Notification statuses.
const (
	StatusNew        = "NEW"
	StatusProcessing = "PROCESSING"
	StatusSending    = "SENDING"
	StatusComplete   = "COMPLETE"
	StatusError      = "ERROR"
	StatusCancelled  = "CANCELLED"
)

// Delivery statuses.
const (
	DeliverySending   = "SENDING"
	DeliverySent      = "SENT"
	DeliveryRetry     = "RETRY"
	DeliveryError     = "ERROR"
	DeliveryCancelled = "CANCELLED"
)

// Audience types.
const (
	AudienceTypeSQL    = "SQL"
	AudienceTypeFilter = "FILTER"
	AudienceTypeGroup  = "GROUP"
	AudienceTypeDirect = "DIRECT"
)

// ValidChannels lists every supported channel.
var ValidChannels = []string{ChannelSMS, ChannelEmail, ChannelPush}

// IsValidChannel reports whether c is a supported channel.
func IsValidChannel(c string) bool {
	return c == ChannelSMS || c == ChannelEmail || c == ChannelPush
}

// IsValidPriority reports whether p is a supported priority.
func IsValidPriority(p string) bool {
	return p == PriorityHigh || p == PriorityNormal || p == PriorityLow
}

// IsValidStatus reports whether s is a notification status.
func IsValidStatus(s string) bool {
	switch s {
	case StatusNew, StatusProcessing, StatusSending, StatusComplete, StatusError, StatusCancelled:
		return true
	}
	return false
}

// AddressColumn returns the master_contact column holding the address used
// by a channel.
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
