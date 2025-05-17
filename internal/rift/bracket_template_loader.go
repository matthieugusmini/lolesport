package rift

import (
	"context"
	"log/slog"
)

// BracketTemplateCache represents a cache to retrieve bracket templates.
type BracketTemplateCache interface {
	// GetBracketTemplate should return:
	// - The BracketTemplate associated to the given stage id that is retrieved from the cache
	// - A boolean indicating whether the value was found or not in the cache
	// - An error(e.g. could not invalidate entry, etc.)
	Get(stageID string) (BracketTemplate, bool, error)

	// SetBracketTemplate should create a new entry in the cache with a
	// rift.BracketTemplate as value and a stageID as key.
	Set(stageID string, value BracketTemplate) error
}

// BracketTemplateClient represents a client to retrieve bracket templates
// performing I/O (e.g. network).
type BracketTemplateClient interface {
	// GetTemplateByStageID shoulds return the BracketTemplate
	// associated to the given stage id.
	GetTemplateByStageID(ctx context.Context, stageID string) (BracketTemplate, error)
	GetAvailableStageTemplate(ctx context.Context) ([]string, error)
}

// BracketTemplateLoader handles loading bracket templates from multiple sources.
type BracketTemplateLoader struct {
	client BracketTemplateClient
	cache  BracketTemplateCache
	logger *slog.Logger
}

// NewBracketTemplateLoader creates a new instance of BracketTemplateLoader.
func NewBracketTemplateLoader(
	bracketTemplateClient BracketTemplateClient,
	cache BracketTemplateCache,
	logger *slog.Logger,
) *BracketTemplateLoader {
	return &BracketTemplateLoader{
		client: bracketTemplateClient,
		cache:  cache,
		logger: logger.WithGroup("bracketTemplateLoader"),
	}
}

// Available stageIDs
func (l *BracketTemplateLoader) ListAvailableStageIDs(ctx context.Context) ([]string, error) {
	// First version without cache

	stageIDs, err := l.client.GetAvailableStageTemplate(ctx)

	if err != nil {
		return nil, err
	}

	return stageIDs, nil
}

// Load tries to load the bracket template associated to the given stage ID
// from the underlying cache first and if not found fetches it using the client.
//
// An error is returned only if the client cannot load the template.
// Errors returned by the cache are not forwarded and are just logged instead.
func (l *BracketTemplateLoader) Load(
	ctx context.Context,
	stageID string,
) (BracketTemplate, error) {
	var tmpl BracketTemplate

	tmpl, ok, err := l.cache.Get(stageID)
	if err != nil {
		l.logger.Debug(
			"Bracket template not present in cache",
			slog.Any("err", err),
			slog.String("stageId", stageID),
		)
	}
	if ok {
		return tmpl, nil
	}

	tmpl, err = l.client.GetTemplateByStageID(ctx, stageID)
	if err != nil {
		return BracketTemplate{}, err
	}

	if err := l.cache.Set(stageID, tmpl); err != nil {
		l.logger.Warn(
			"Failed to cache bracket template",
			slog.Any("err", err),
			slog.String("stageId", stageID),
		)
	}

	return tmpl, nil
}
