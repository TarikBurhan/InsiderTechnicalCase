package dto

// QueueStat is the depth of a RabbitMQ queue.
type QueueStat struct {
	Name      string
	Messages  int
	Consumers int
	Error     string
}

// ChannelDeliveryStat aggregates delivery outcomes of a channel in a window.
type ChannelDeliveryStat struct {
	Channel      string
	Sent         int64
	Failed       int64
	Retrying     int64
	InFlight     int64
	AvgLatencyMs float64
	P95LatencyMs float64
}

// StatusCount is a notification count per status.
type StatusCount struct {
	Status string
	Count  int64
}
