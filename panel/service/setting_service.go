package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"notifier/panel/dto"
	"notifier/panel/repository"
	"notifier/panel/validation"
	"notifier/pkg/timezone"
)

// TimeStatus describes how the account time zone is applied right now.
type TimeStatus struct {
	AccountTimezone string
	ServiceTimezone string
	ContainerTZ     string
	Database        dto.DatabaseTime
	ServiceNow      time.Time
	ClockDrift      time.Duration
	Problems        []string
}

// SettingService manages account settings.
type SettingService struct {
	repo   *repository.SettingRepository
	holder *timezone.Holder
	log    *slog.Logger
}

// NewSettingService creates the service.
func NewSettingService(repo *repository.SettingRepository, holder *timezone.Holder, log *slog.Logger) *SettingService {
	return &SettingService{repo: repo, holder: holder, log: log}
}

// List returns every setting.
func (s *SettingService) List(ctx context.Context) ([]dto.Setting, error) { return s.repo.List(ctx) }

// Get returns one setting.
func (s *SettingService) Get(ctx context.Context, key string) (dto.Setting, error) {
	return s.repo.Get(ctx, key)
}

// Update validates and stores a setting. Only known keys can be changed.
func (s *SettingService) Update(ctx context.Context, key, value string) (dto.Setting, string, error) {
	value = strings.TrimSpace(value)
	note := ""
	switch key {
	case dto.SettingTimezone:
		if _, err := time.LoadLocation(value); err != nil || value == "" || strings.EqualFold(value, "local") {
			return dto.Setting{}, "", validation.New("value", fmt.Sprintf("%q is not a valid IANA time zone (e.g. Europe/Istanbul)", value))
		}
		ok, err := s.repo.TimezoneExists(ctx, value)
		if err != nil {
			return dto.Setting{}, "", err
		}
		if !ok {
			return dto.Setting{}, "", validation.New("value", fmt.Sprintf("PostgreSQL does not know time zone %q", value))
		}
		note = "database default updated; every service re-reads the setting and restarts within the watch interval to apply it"
	default:
		if _, err := s.repo.Get(ctx, key); err != nil {
			return dto.Setting{}, "", err
		}
		return dto.Setting{}, "", validation.New("key", fmt.Sprintf("setting %q is read-only", key))
	}

	out, err := s.repo.Update(ctx, key, value)
	if err != nil {
		return dto.Setting{}, "", err
	}
	s.log.InfoContext(ctx, "account setting updated", slog.String("key", key), slog.String("value", value))
	return out, note, nil
}

// TimeStatus compares the account time zone with the service, the container
// and the database and reports every inconsistency.
func (s *SettingService) TimeStatus(ctx context.Context) (TimeStatus, error) {
	account, err := s.repo.Get(ctx, dto.SettingTimezone)
	if err != nil {
		return TimeStatus{}, err
	}
	db, err := s.repo.DatabaseTime(ctx)
	if err != nil {
		return TimeStatus{}, err
	}
	st := TimeStatus{
		AccountTimezone: account.Value,
		ServiceTimezone: s.holder.Name(),
		ContainerTZ:     os.Getenv("TZ"),
		Database:        db,
		ServiceNow:      time.Now(),
		ClockDrift:      timezone.Drift(db.Now),
	}
	if st.ServiceTimezone != st.AccountTimezone {
		st.Problems = append(st.Problems, "service has not applied the current account timezone yet (restart pending)")
	}
	if db.SessionTimezone != st.AccountTimezone {
		st.Problems = append(st.Problems, "database session timezone differs from account timezone")
	}
	if db.DatabaseTimezone != st.AccountTimezone {
		st.Problems = append(st.Problems, "database default timezone differs from account timezone")
	}
	if st.ContainerTZ != "" && st.ContainerTZ != st.AccountTimezone {
		st.Problems = append(st.Problems, "container TZ differs from account timezone (account timezone is used)")
	}
	if st.ClockDrift > timezone.MaxClockDrift {
		st.Problems = append(st.Problems, fmt.Sprintf("clock drift with database is %s", st.ClockDrift))
	}
	return st, nil
}
