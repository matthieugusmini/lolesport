package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/matthieugusmini/rift/internal/rift"
)

const (
	baseURL = "https://raw.githubusercontent.com/matthieugusmini/lolesports-bracket-templates/refs/heads/main/"

	bracketTypeByStageIDFilename = "bracket-type-by-stage-id.json"
)

// BracketTemplateClient handles fetching bracket templates from JSON config files
// stored in a GitHub repository.
//
// Example: https://raw.githubusercontent.com/matthieugusmini/lolesports-bracket-templates/refs/heads/main/8SE.json
type BracketTemplateClient struct {
	httpClient *http.Client
}

// NewBracketTemplateClient creates a new instance of BracketTemplateClient.
func NewBracketTemplateClient(httpClient *http.Client) *BracketTemplateClient {
	return &BracketTemplateClient{
		httpClient: httpClient,
	}
}

func (c *BracketTemplateClient) GetAvailableStageTemplate(ctx context.Context) ([]string, error) {
	var (
		bracketTypeByStageID map[string]string
	)
	bracketTypeByStageID, err := c.getBracketTemplateMapper(ctx)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(bracketTypeByStageID))
	for k := range bracketTypeByStageID {
		keys = append(keys, k)
	}

	return keys, nil
}

// GetTemplateByStageID fetches the bracket template associated with the given stage id.
//
// An error is returned if no bracket template is associated with the stage id or
// in case of HTTP error.
func (c *BracketTemplateClient) GetTemplateByStageID(
	ctx context.Context,
	stageID string,
) (rift.BracketTemplate, error) {
	var (
		bracketTypeByStageID map[string]string
		data                 rift.BracketTemplate
	)
	bracketTypeByStageID, err := c.getBracketTemplateMapper(ctx)
	if err != nil {
		return rift.BracketTemplate{}, err
	}

	bracketType, ok := bracketTypeByStageID[stageID]
	if !ok {
		return rift.BracketTemplate{}, fmt.Errorf("stage ID %q is unsupported", stageID)
	}

	bracketTemplateURL := fmt.Sprintf("%s%s.json", baseURL, bracketType)
	if err := c.get(ctx, bracketTemplateURL, &data); err != nil {
		return rift.BracketTemplate{}, err
	}

	return data, nil
}

func (c *BracketTemplateClient) getBracketTemplateMapper(
	ctx context.Context,
) (map[string]string, error) {
	var data map[string]string

	bracketTypeByStageIDURL := baseURL + bracketTypeByStageIDFilename

	if err := c.get(ctx, bracketTypeByStageIDURL, &data); err != nil {
		return map[string]string{}, err
	}

	return data, nil
}

func (c *BracketTemplateClient) get(ctx context.Context, url string, data any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("could not create new request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(data)
}
