package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"notifier/panel/dto"
	"notifier/panel/repository"
	"notifier/panel/request"
	"notifier/panel/validation"
)

const schemaCacheTTL = 5 * time.Minute

// AudienceConfig tunes audience validation.
type AudienceConfig struct {
	ValidationTimeout   time.Duration
	ExtraTables         []string
	MaxDirectRecipients int
}

// AudienceService validates audiences and manages audience groups.
type AudienceService struct {
	repo *repository.AudienceRepository
	cfg  AudienceConfig

	mu       sync.RWMutex
	schema   dto.TableSchema
	loadedAt time.Time
}

// NewAudienceService creates the service.
func NewAudienceService(repo *repository.AudienceRepository, cfg AudienceConfig) *AudienceService {
	return &AudienceService{repo: repo, cfg: cfg}
}

// Schema returns (cached) column metadata of the tables audiences may use.
func (s *AudienceService) Schema(ctx context.Context) (dto.TableSchema, error) {
	s.mu.RLock()
	if s.schema != nil && time.Since(s.loadedAt) < schemaCacheTTL {
		defer s.mu.RUnlock()
		return s.schema, nil
	}
	s.mu.RUnlock()

	tables := append([]string{validation.MasterContactTable}, s.cfg.ExtraTables...)
	schema, err := s.repo.TableSchema(ctx, tables)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.schema, s.loadedAt = schema, time.Now()
	s.mu.Unlock()
	return schema, nil
}

// ContactColumns returns the master_contact columns usable as template variables.
func (s *AudienceService) ContactColumns(ctx context.Context) (map[string]string, error) {
	schema, err := s.Schema(ctx)
	if err != nil {
		return nil, err
	}
	return schema[validation.MasterContactTable], nil
}

// Resolve validates an audience request for a channel. memo caches counts
// of identical SQL inside one (batch) request.
func (s *AudienceService) Resolve(ctx context.Context, channel string, req request.AudienceRequest, memo map[string]int64) (dto.ResolvedAudience, error) {
	definition, _ := json.Marshal(req)
	out := dto.ResolvedAudience{Type: strings.ToUpper(strings.TrimSpace(req.Type)), Definition: definition}

	switch out.Type {
	case dto.AudienceTypeSQL:
		sql, err := validation.SanitizeAudienceSQL(req.SQL)
		if err != nil {
			return out, err
		}
		count, err := s.countWithMemo(ctx, sql, memo)
		if err != nil {
			return out, err
		}
		out.SQL, out.Count = &sql, count

	case dto.AudienceTypeFilter:
		sql, count, err := s.buildFilter(ctx, req.Filter, memo)
		if err != nil {
			return out, err
		}
		out.SQL, out.Count = &sql, count

	case dto.AudienceTypeGroup:
		if req.GroupID == nil {
			return out, validation.New("audience.group_id", "group_id is required for GROUP audiences")
		}
		// Groups were validated when they were saved: no re-validation here.
		g, err := s.repo.GetGroup(ctx, *req.GroupID)
		if errors.Is(err, repository.ErrNotFound) {
			return out, validation.New("audience.group_id", fmt.Sprintf("audience group %d not found", *req.GroupID))
		}
		if err != nil {
			return out, err
		}
		out.SQL, out.GroupID, out.Count = &g.SQLQuery, &g.ID, g.ContactCount

	case dto.AudienceTypeDirect:
		recipients, err := validation.NormalizeRecipients(channel, req.Recipients, s.cfg.MaxDirectRecipients)
		if err != nil {
			return out, err
		}
		out.Recipients, out.Count = recipients, int64(len(recipients))

	default:
		return out, validation.New("audience.type", "type must be one of SQL, FILTER, GROUP, DIRECT")
	}
	return out, nil
}

func (s *AudienceService) buildFilter(ctx context.Context, f *request.AudienceFilter, memo map[string]int64) (string, int64, error) {
	if f == nil {
		return "", 0, validation.New("audience.filter", "filter is required for FILTER audiences")
	}
	schema, err := s.Schema(ctx)
	if err != nil {
		return "", 0, err
	}
	sql, err := validation.BuildFilterSQL(*f, schema)
	if err != nil {
		return "", 0, err
	}
	count, err := s.countWithMemo(ctx, sql, memo)
	return sql, count, err
}

func (s *AudienceService) countWithMemo(ctx context.Context, sql string, memo map[string]int64) (int64, error) {
	if memo != nil {
		if c, ok := memo[sql]; ok {
			return c, nil
		}
	}
	count, err := s.repo.CountAudience(ctx, sql, s.cfg.ValidationTimeout)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return 0, validation.New("audience", "audience query rejected by database: "+pgErr.Message)
		}
		return 0, err
	}
	if count == 0 {
		return 0, validation.New("audience", "audience does not match any master_contact record")
	}
	if memo != nil {
		memo[sql] = count
	}
	return count, nil
}

// Validate dry-runs an audience definition.
func (s *AudienceService) Validate(ctx context.Context, req request.ValidateAudienceRequest) (dto.ResolvedAudience, error) {
	if !dto.IsValidChannel(req.Channel) {
		return dto.ResolvedAudience{}, validation.New("channel", "channel must be one of sms, email, push")
	}
	return s.Resolve(ctx, req.Channel, req.Audience, nil)
}

// CreateGroup validates and saves an audience group. Validation happens
// here once, so notifications using the group skip it.
func (s *AudienceService) CreateGroup(ctx context.Context, req request.CreateAudienceGroupRequest) (dto.AudienceGroup, error) {
	var errs validation.Errors
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		errs.Add("name", "name is required (max 100 characters)")
	}
	t := strings.ToUpper(strings.TrimSpace(req.Type))
	if t != dto.AudienceTypeSQL && t != dto.AudienceTypeFilter {
		errs.Add("type", "type must be SQL or FILTER")
	}
	if len(errs) > 0 {
		return dto.AudienceGroup{}, errs
	}

	resolved, err := s.Resolve(ctx, "", request.AudienceRequest{Type: t, SQL: req.SQL, Filter: req.Filter}, nil)
	if err != nil {
		return dto.AudienceGroup{}, err
	}
	var desc *string
	if d := strings.TrimSpace(req.Description); d != "" {
		desc = &d
	}
	var definition json.RawMessage
	if req.Filter != nil {
		definition, _ = json.Marshal(req.Filter)
	}
	g, err := s.repo.InsertGroup(ctx, dto.AudienceGroup{
		Name: name, Description: desc, SourceType: t, Definition: definition,
		SQLQuery: *resolved.SQL, ContactCount: resolved.Count,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return dto.AudienceGroup{}, conflictf("audience group %q already exists", name)
	}
	return g, err
}

// GetGroup loads a group.
func (s *AudienceService) GetGroup(ctx context.Context, id int64) (dto.AudienceGroup, error) {
	return s.repo.GetGroup(ctx, id)
}

// ListGroups pages through groups.
func (s *AudienceService) ListGroups(ctx context.Context, limit, offset int) ([]dto.AudienceGroup, int64, error) {
	return s.repo.ListGroups(ctx, limit, offset)
}

// DeleteGroup deletes a group.
func (s *AudienceService) DeleteGroup(ctx context.Context, id int64) error {
	return s.repo.DeleteGroup(ctx, id)
}
