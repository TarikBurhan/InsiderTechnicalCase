package dto

// AudienceContact is one resolved audience member.
type AudienceContact struct {
	ContactID *int64
	Address   string
}

// DeliveryTarget is a delivery row ready to be produced to a queue.
type DeliveryTarget struct {
	DeliveryID int64
	ContactID  *int64
	Recipient  string
	Attempt    int // 0 = first attempt (the batch attempt is used)
}

// RetryDelivery is a delivery whose retry time has come.
type RetryDelivery struct {
	DeliveryTarget
	NotificationID string
	RetryCount     int
}

// SweptDelivery is a stuck delivery moved back to RETRY.
type SweptDelivery struct {
	DeliveryID     int64
	NotificationID string
	TraceContext   string
}
