package service

import (
	"context"
	"time"

	"notifier/panel/dto"
	"notifier/panel/repository"
	"notifier/pkg/rabbitmq"
)

// QueueInspector returns queue depth information.
type QueueInspector interface {
	QueueDepth(name string) (messages int, consumers int, err error)
}

// MetricsSnapshot is the data behind the real-time metrics endpoint.
type MetricsSnapshot struct {
	GeneratedAt   time.Time
	WindowSeconds int
	Queues        []dto.QueueStat
	Channels      []dto.ChannelDeliveryStat
	StatusCounts  []dto.StatusCount
}

// MetricsService builds real-time metrics from RabbitMQ and PostgreSQL.
type MetricsService struct {
	repo   *repository.MetricsRepository
	queues QueueInspector
}

// NewMetricsService creates the service.
func NewMetricsService(repo *repository.MetricsRepository, queues QueueInspector) *MetricsService {
	return &MetricsService{repo: repo, queues: queues}
}

// Snapshot collects queue depths, per channel success/failure/latency and
// notification status counts.
func (s *MetricsService) Snapshot(ctx context.Context, windowSeconds int) (MetricsSnapshot, error) {
	snap := MetricsSnapshot{GeneratedAt: time.Now().UTC(), WindowSeconds: windowSeconds}

	names := make([]string, 0, len(rabbitmq.AllQueues)*2)
	for _, q := range rabbitmq.AllQueues {
		names = append(names, q, rabbitmq.DLQName(q))
	}
	for _, name := range names {
		qs := dto.QueueStat{Name: name}
		if msgs, consumers, err := s.queues.QueueDepth(name); err != nil {
			qs.Error = err.Error()
		} else {
			qs.Messages, qs.Consumers = msgs, consumers
		}
		snap.Queues = append(snap.Queues, qs)
	}

	for _, ch := range dto.ValidChannels {
		stat, err := s.repo.ChannelStats(ctx, ch, windowSeconds)
		if err != nil {
			return snap, err
		}
		snap.Channels = append(snap.Channels, stat)
	}

	counts, err := s.repo.StatusCounts(ctx)
	if err != nil {
		return snap, err
	}
	snap.StatusCounts = counts
	return snap, nil
}
