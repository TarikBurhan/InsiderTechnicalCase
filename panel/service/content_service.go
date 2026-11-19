package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"notifier/panel/dto"
	"notifier/panel/repository"
	"notifier/panel/request"
	"notifier/panel/validation"
)

// ContentService manages reusable contents and resolves notification content.
type ContentService struct {
	repo     *repository.ContentRepository
	audience *AudienceService
}

// NewContentService creates the service.
func NewContentService(repo *repository.ContentRepository, audience *AudienceService) *ContentService {
	return &ContentService{repo: repo, audience: audience}
}

func toMessageContent(p request.ContentPayload) (dto.MessageContent, error) {
	mc := dto.MessageContent{Subject: p.Subject, Title: p.Title, Body: p.Body, IsHTML: p.IsHTML, ImageURL: p.ImageURL}
	if len(p.Data) > 0 {
		raw, err := json.Marshal(p.Data)
		if err != nil {
			return mc, validation.New("content.data", "must be a JSON object")
		}
		mc.Data = raw
	}
	return mc, nil
}

func (s *ContentService) validate(ctx context.Context, channel string, mc dto.MessageContent, prefix string) ([]string, error) {
	errs := validation.ValidateContent(channel, mc, prefix)
	vars := validation.ExtractVariables(mc.Body, mc.Subject, mc.Title)
	if len(vars) > 0 {
		cols, err := s.audience.ContactColumns(ctx)
		if err != nil {
			return nil, err
		}
		errs = append(errs, validation.ValidateVariables(vars, cols, strings.TrimSuffix(prefix, "."))...)
	}
	return vars, errs.Err()
}

// Create validates and saves a reusable content for a channel.
func (s *ContentService) Create(ctx context.Context, channel string, req request.CreateContentRequest) (dto.Content, error) {
	if !dto.IsValidChannel(channel) {
		return dto.Content{}, validation.New("channel", "channel must be one of sms, email, push")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > validation.ContentNameMaxLength {
		return dto.Content{}, validation.New("name", "name is required (max 100 characters)")
	}
	isHTML := req.IsHTML != nil && *req.IsHTML
	if req.IsHTML == nil && channel == dto.ChannelEmail {
		isHTML = true
	}
	mc, err := toMessageContent(request.ContentPayload{
		Subject: req.Subject, Title: req.Title, Body: req.Body, IsHTML: isHTML, ImageURL: req.ImageURL, Data: req.Data,
	})
	if err != nil {
		return dto.Content{}, err
	}
	vars, err := s.validate(ctx, channel, mc, "")
	if err != nil {
		return dto.Content{}, err
	}

	c := dto.Content{Channel: channel, Name: name, Body: mc.Body, IsHTML: isHTML, Data: mc.Data, Variables: vars}
	if channel == dto.ChannelEmail {
		c.Subject = &mc.Subject
	}
	if channel == dto.ChannelPush {
		c.Title = &mc.Title
		if mc.ImageURL != "" {
			c.ImageURL = &mc.ImageURL
		}
	}
	out, err := s.repo.Insert(ctx, c)
	if errors.Is(err, repository.ErrDuplicate) {
		return dto.Content{}, conflictf("%s content %q already exists", channel, name)
	}
	return out, err
}

// Get loads a content.
func (s *ContentService) Get(ctx context.Context, channel string, id int64) (dto.Content, error) {
	if !dto.IsValidChannel(channel) {
		return dto.Content{}, ErrNotFound
	}
	return s.repo.Get(ctx, channel, id)
}

// List pages through contents of a channel.
func (s *ContentService) List(ctx context.Context, channel string, limit, offset int) ([]dto.Content, int64, error) {
	if !dto.IsValidChannel(channel) {
		return nil, 0, ErrNotFound
	}
	return s.repo.List(ctx, channel, limit, offset)
}

// Delete removes a content.
func (s *ContentService) Delete(ctx context.Context, channel string, id int64) error {
	if !dto.IsValidChannel(channel) {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, channel, id)
}

// ResolveForNotification returns the content snapshot for a notification:
// either a stored (already validated) content or a freshly validated inline
// content. It also enforces the customized_content flag rules.
func (s *ContentService) ResolveForNotification(ctx context.Context, channel string, contentID *int64,
	inline *request.ContentPayload, customized bool) (dto.MessageContent, error) {

	var (
		mc   dto.MessageContent
		vars []string
	)
	switch {
	case contentID != nil && inline != nil:
		return mc, validation.New("content", "use either content_id or content, not both")
	case contentID != nil:
		c, err := s.repo.Get(ctx, channel, *contentID)
		if errors.Is(err, repository.ErrNotFound) {
			return mc, validation.New("content_id", fmt.Sprintf("%s content %d not found", channel, *contentID))
		}
		if err != nil {
			return mc, err
		}
		mc, vars = c.ToMessageContent(), c.Variables
	case inline != nil:
		var err error
		if mc, err = toMessageContent(*inline); err != nil {
			return mc, err
		}
		if vars, err = s.validate(ctx, channel, mc, "content."); err != nil {
			return mc, err
		}
	default:
		return mc, validation.New("content", "content_id or content is required")
	}

	if len(vars) > 0 && !customized {
		return mc, validation.New("customized_content",
			fmt.Sprintf("content uses variables %v; set customized_content=true", vars))
	}
	if customized && len(vars) == 0 {
		return mc, validation.New("customized_content", "customized_content=true but content has no {{variables}}")
	}
	return mc, nil
}
