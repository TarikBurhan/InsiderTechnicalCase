package dto

// Delivery statuses.
const (
	DeliverySent  = "SENT"
	DeliveryRetry = "RETRY"
	DeliveryError = "ERROR"
)

// SendResult is the outcome of one provider call.
type SendResult struct {
	DeliveryID        int64
	Recipient         string
	ProviderMessageID string
	Err               error
	Permanent         bool
	LatencyMs         int64
}

// FailedDelivery is a failed delivery after its status was updated.
type FailedDelivery struct {
	DeliveryID int64
	Status     string
	RetryCount int
	Error      string
}

// LogEntry is one notification_logs row.
type LogEntry struct {
	NotificationID string
	DeliveryID     *int64
	Level          string
	Event          string
	Message        string
	Details        []byte
}
