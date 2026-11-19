package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"notifier/panel/dto"
	"notifier/panel/repository"
	"notifier/panel/request"
	"notifier/panel/validation"
	"notifier/pkg/correlation"
)

const (
	maxIdempotencyKeyLength = 255
	maxScheduleAhead        = 365 * 24 * time.Hour
)

// NotificationConfig tunes the notification service.
type NotificationConfig struct {
	MaxBatchSize int
}

// CreateResult is the outcome of creating one notification.
type CreateResult struct {
	Notification  dto.Notification
	Created       bool
	AudienceCount int64
}

// NotificationService handles notification requests coming from the API.
type NotificationService struct {
	repo       *repository.NotificationRepository
	deliveries *repository.DeliveryRepository
	logs       *repository.LogRepository
	audience   *AudienceService
	contents   *ContentService
	cfg        NotificationConfig
	log        *slog.Logger
}

// NewNotificationService creates the service.
func NewNotificationService(repo *repository.NotificationRepository, deliveries *repository.DeliveryRepository,
	logs *repository.LogRepository, audience *AudienceService, contents *ContentService,
	cfg NotificationConfig, log *slog.Logger) *NotificationService {
	return &NotificationService{repo: repo, deliveries: deliveries, logs: logs, audience: audience,
		contents: contents, cfg: cfg, log: log}
}

// prepare validates a request and turns it into an insertable notification.
func (s *NotificationService) prepare(ctx context.Context, req request.CreateNotificationRequest, memo map[string]int64) (dto.NewNotification, int64, error) {
	var errs validation.Errors
	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if !dto.IsValidChannel(channel) {
		errs.Add("channel", "channel must be one of sms, email, push")
	}
	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = dto.PriorityNormal
	}
	if !dto.IsValidPriority(priority) {
		errs.Add("priority", "priority must be one of high, normal, low")
	}
	if len(req.IdempotencyKey) > maxIdempotencyKeyLength {
		errs.Add("idempotency_key", "must be at most 255 characters")
	}
	if req.ScheduledAt != nil && time.Until(req.ScheduledAt.Time) > maxScheduleAhead {
		errs.Add("scheduled_at", "cannot be more than one year in the future")
	}
	if req.CustomizedContent && strings.EqualFold(req.Audience.Type, dto.AudienceTypeDirect) {
		errs.Add("customized_content", "customized content requires a contact based audience (SQL, FILTER or GROUP)")
	}
	if len(errs) > 0 {
		return dto.NewNotification{}, 0, errs
	}

	content, err := s.contents.ResolveForNotification(ctx, channel, req.ContentID, req.Content, req.CustomizedContent)
	if err != nil {
		return dto.NewNotification{}, 0, err
	}
	audience, err := s.audience.Resolve(ctx, channel, req.Audience, memo)
	if err != nil {
		return dto.NewNotification{}, 0, err
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return dto.NewNotification{}, 0, err
	}
	n := dto.NewNotification{
		CorrelationID:      correlation.FromContext(ctx),
		Channel:            channel,
		Priority:           priority,
		AudienceType:       audience.Type,
		AudienceGroupID:    audience.GroupID,
		AudienceDefinition: audience.Definition,
		AudienceSQL:        audience.SQL,
		ContentID:          req.ContentID,
		Content:            contentJSON,
		CustomizedContent:  req.CustomizedContent,
		ScheduledAt:        req.ScheduledAt.Ptr(),
	}
	if audience.Type == dto.AudienceTypeDirect {
		if n.Recipients, err = json.Marshal(audience.Recipients); err != nil {
			return dto.NewNotification{}, 0, err
		}
	}
	if key := strings.TrimSpace(req.IdempotencyKey); key != "" {
		n.IdempotencyKey = &key
	}
	return n, audience.Count, nil
}

// Create validates and stores a single notification as NEW. Re-using an
// idempotency key returns the original notification (Created=false).
func (s *NotificationService) Create(ctx context.Context, req request.CreateNotificationRequest) (CreateResult, error) {
	if key := strings.TrimSpace(req.IdempotencyKey); key != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, key)
		if err == nil {
			return CreateResult{Notification: existing, Created: false, AudienceCount: int64(existing.TotalRecipients)}, nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return CreateResult{}, err
		}
	}

	n, count, err := s.prepare(ctx, req, nil)
	if err != nil {
		return CreateResult{}, err
	}
	saved, created, err := s.repo.Insert(ctx, n)
	if err != nil {
		return CreateResult{}, err
	}
	if created {
		s.writeLog(ctx, saved, "NOTIFICATION_CREATED", "notification request accepted",
			map[string]any{"audience_type": saved.AudienceType, "audience_count": count, "priority": saved.Priority})
	}
	return CreateResult{Notification: saved, Created: created, AudienceCount: count}, nil
}

// CreateBatch validates every item first (all-or-nothing) and inserts them
// in a single transaction sharing one batch id.
func (s *NotificationService) CreateBatch(ctx context.Context, req request.BatchCreateNotificationRequest) (string, []CreateResult, error) {
	if len(req.Notifications) == 0 {
		return "", nil, validation.New("notifications", "at least one notification is required")
	}
	if len(req.Notifications) > s.cfg.MaxBatchSize {
		return "", nil, validation.New("notifications", fmt.Sprintf("at most %d notifications per batch", s.cfg.MaxBatchSize))
	}

	batchID := uuid.NewString()
	memo := map[string]int64{}
	items := make([]dto.NewNotification, len(req.Notifications))
	counts := make([]int64, len(req.Notifications))
	var errs validation.Errors
	for i, r := range req.Notifications {
		n, count, err := s.prepare(ctx, r, memo)
		if err != nil {
			var verrs validation.Errors
			if errors.As(err, &verrs) {
				errs = append(errs, verrs.Prefix(fmt.Sprintf("notifications[%d].", i))...)
				continue
			}
			return "", nil, err
		}
		n.BatchID = &batchID
		items[i], counts[i] = n, count
	}
	if len(errs) > 0 {
		return "", nil, errs
	}

	saved, created, err := s.repo.InsertBatch(ctx, items)
	if err != nil {
		return "", nil, err
	}
	results := make([]CreateResult, len(saved))
	for i := range saved {
		results[i] = CreateResult{Notification: saved[i], Created: created[i], AudienceCount: counts[i]}
	}
	s.writeLogBatch(ctx, results, batchID)
	return batchID, results, nil
}

// Get returns a notification with its delivery statistics.
func (s *NotificationService) Get(ctx context.Context, id string) (dto.Notification, dto.DeliveryStats, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return n, dto.DeliveryStats{}, err
	}
	stats, err := s.deliveries.Stats(ctx, n.Channel, n.ID)
	return n, stats, err
}

// GetBatch returns every notification of a batch.
func (s *NotificationService) GetBatch(ctx context.Context, batchID string) ([]dto.Notification, error) {
	items, err := s.repo.ListByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	return items, nil
}

// List returns a filtered page of notifications.
func (s *NotificationService) List(ctx context.Context, f dto.NotificationFilter) ([]dto.Notification, int64, error) {
	return s.repo.List(ctx, f)
}

// Cancel cancels a notification that has not completed yet. Pending
// deliveries are cancelled too, so senders and the retry scheduler skip them.
func (s *NotificationService) Cancel(ctx context.Context, id string) (dto.Notification, error) {
	n, err := s.repo.Cancel(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		existing, getErr := s.repo.GetByID(ctx, id)
		if getErr != nil {
			return dto.Notification{}, getErr
		}
		return dto.Notification{}, conflictf("notification in status %s cannot be cancelled", existing.Status)
	}
	if err != nil {
		return dto.Notification{}, err
	}
	s.writeLog(ctx, n, "NOTIFICATION_CANCELLED", "notification cancelled via API", nil)
	return n, nil
}

// Deliveries pages through the deliveries of a notification.
func (s *NotificationService) Deliveries(ctx context.Context, id, status string, limit, offset int) ([]dto.Delivery, int64, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	return s.deliveries.List(ctx, n.Channel, n.ID, strings.ToUpper(status), limit, offset)
}

// Logs pages through the logs of a notification.
func (s *NotificationService) Logs(ctx context.Context, id string, limit, offset int) ([]dto.NotificationLog, int64, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.logs.List(ctx, id, limit, offset)
}

func (s *NotificationService) writeLog(ctx context.Context, n dto.Notification, event, message string, details any) {
	if err := s.logs.Write(ctx, &n.ID, n.Channel, "INFO", event, message, correlation.FromContext(ctx), details); err != nil {
		s.log.WarnContext(ctx, "could not write notification log", slog.String("event", event), slog.Any("error", err))
	}
}

func (s *NotificationService) writeLogBatch(ctx context.Context, results []CreateResult, batchID string) {
	for _, r := range results {
		if r.Created {
			s.writeLog(ctx, r.Notification, "NOTIFICATION_CREATED", "notification request accepted (batch)",
				map[string]any{"batch_id": batchID, "audience_count": r.AudienceCount})
		}
	}
}
