package rift_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/matthieugusmini/rift/internal/rift"
)

func TestBracketTemplateLoader_Load(t *testing.T) {
	stageID := "Chunin Exams"
	want := chuninExamsBracketTemplate

	t.Run("when template present in cache returns bracket template", func(t *testing.T) {
		fakeCache := newFakeCacheWith(map[string]rift.BracketTemplate{stageID: want})
		stubAPIClient := newStubBracketTemplateAPIClient()
		loader := rift.NewBracketTemplateLoader(stubAPIClient, fakeCache, slog.Default())

		got := mustLoad(t, loader, stageID)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf(
				"BracketTemplateLoader.Load(stageID) returned unexpected diffs(-want +got):\n%s",
				diff,
			)
		}
	})

	t.Run(
		"when template not in cache fetch from API and returns bracket template and set value in cache",
		func(t *testing.T) {
			fakeCache := newFakeCache[rift.BracketTemplate]()
			stubAPIClient := newStubBracketTemplateAPIClient()
			loader := rift.NewBracketTemplateLoader(stubAPIClient, fakeCache, slog.Default())

			got := mustLoad(t, loader, stageID)

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf(
					"BracketTemplateLoader.Load(stageID) returned unexpected diffs(-want +got):\n%s",
					diff,
				)
			}

			// Assert that the cache has been updated
			cacheEntry, ok := fakeCache.entries[stageID]
			if !ok {
				t.Fatalf("Bracket template should be cached after loading")
			}
			if diff := cmp.Diff(want, cacheEntry); diff != "" {
				t.Errorf("Cache[stageID] has unexpected diffs(-want +got):\n%s", diff)
			}
		},
	)

	t.Run("when template not in cache and not found in API returns error", func(t *testing.T) {
		fakeCache := newFakeCache[rift.BracketTemplate]()
		notFoundAPIClient := newNotFoundBracketTemplateAPIClient()
		loader := rift.NewBracketTemplateLoader(notFoundAPIClient, fakeCache, slog.Default())

		_, err := loader.Load(t.Context(), stageID)
		if err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run(
		"when fails to get in cache should log and returns bracket template",
		func(t *testing.T) {
			fakeCache := &fakeCache[rift.BracketTemplate]{
				entries: map[string]rift.BracketTemplate{},
				getErr:  errCacheGet,
			}
			stubAPIClient := newStubBracketTemplateAPIClient()
			loader := rift.NewBracketTemplateLoader(stubAPIClient, fakeCache, slog.Default())

			got := mustLoad(t, loader, stageID)

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf(
					"BracketTemplateLoader.Load(stageID) returned unexpected diffs(-want +got):\n%s",
					diff,
				)
			}

			// Assert that the cache has been updated
			cacheEntry, ok := fakeCache.entries[stageID]
			if !ok {
				t.Fatalf("Bracket template should be cached after loading")
			}
			if diff := cmp.Diff(want, cacheEntry); diff != "" {
				t.Errorf("Cache[stageID] has unexpected diffs(-want +got):\n%s", diff)
			}
		},
	)

	t.Run("when fails to update cache should log and return bracket template", func(t *testing.T) {
		fakeCache := &fakeCache[rift.BracketTemplate]{
			entries: map[string]rift.BracketTemplate{},
			setErr:  errCacheSet,
		}
		stubAPIClient := newStubBracketTemplateAPIClient()
		loader := rift.NewBracketTemplateLoader(stubAPIClient, fakeCache, slog.Default())

		got := mustLoad(t, loader, stageID)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf(
				"BracketTemplateLoader.Load(stageID) returned unexpected diffs(-want +got):\n%s",
				diff,
			)
		}
	})
}

func mustLoad(
	t *testing.T,
	loader *rift.BracketTemplateLoader,
	stageID string,
) rift.BracketTemplate {
	t.Helper()

	got, err := loader.Load(t.Context(), stageID)
	if err != nil {
		t.Fatalf("got unexpected error %q, want nil", err)
	}

	return got
}

var chuninExamsBracketTemplate = rift.BracketTemplate{
	Rounds: []rift.Round{
		{
			Title: "Forest of Death",
			Links: []rift.Link{
				{Type: rift.LinkTypeZDown, Height: 42},
			},
			Matches: []rift.Match{
				{DisplayType: rift.DisplayTypeMatch, Above: 42},
			},
		},
	},
}

var errAPINotFound = errors.New("not found")

type stubBracketTemplateAPIClient struct {
	template        rift.BracketTemplate
	availableStages []string
	err             error
}

func newStubBracketTemplateAPIClient() *stubBracketTemplateAPIClient {
	return &stubBracketTemplateAPIClient{
		template:        chuninExamsBracketTemplate,
		availableStages: []string{"1", "2"},
	}
}

func newNotFoundBracketTemplateAPIClient() *stubBracketTemplateAPIClient {
	return &stubBracketTemplateAPIClient{err: errAPINotFound}
}

func (api *stubBracketTemplateAPIClient) GetTemplateByStageID(
	_ context.Context,
	_ string,
) (rift.BracketTemplate, error) {
	if api.err != nil {
		return rift.BracketTemplate{}, api.err
	}
	return api.template, nil
}

func (api *stubBracketTemplateAPIClient) GetAvailableStageTemplate(
	_ context.Context,
) ([]string, error) {
	if api.err != nil {
		return nil, api.err
	}
	return api.availableStages, nil
}
