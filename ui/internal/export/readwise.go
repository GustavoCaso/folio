package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

const readwiseBaseURL = "https://readwise.io/api/v2"

type ReadwiseBackend struct {
	apiToken string
	client   *http.Client
}

func newReadwise(config map[string]string) (Backend, error) {
	token, ok := config["api_token"]
	if !ok || token == "" {
		return nil, fmt.Errorf("readwise: api_token is required")
	}
	return &ReadwiseBackend{
		apiToken: token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (r *ReadwiseBackend) Name() string { return "readwise" }

type readwiseHighlightInput struct {
	Text          string `json:"text"`
	Title         string `json:"title"`
	Note          string `json:"note,omitempty"`
	HighlightedAt string `json:"highlighted_at,omitempty"`
}

type readwiseCreateRequest struct {
	Highlights []readwiseHighlightInput `json:"highlights"`
}

type readwiseHighlightResponse struct {
	ID int64 `json:"id"`
}

type readwiseCreateResponse struct {
	Highlights []readwiseHighlightResponse `json:"highlights"`
}

func (r *ReadwiseBackend) Export(ctx context.Context, highlights []db.HighlightWithJob) ([]ExportResult, error) {
	inputs := make([]readwiseHighlightInput, len(highlights))
	for i, h := range highlights {
		inputs[i] = readwiseHighlightInput{
			Text:          h.Text,
			Title:         h.JobFilename,
			Note:          h.Note,
			HighlightedAt: h.CreatedAt.Format(time.RFC3339),
		}
	}

	body, err := json.Marshal(readwiseCreateRequest{Highlights: inputs})
	if err != nil {
		return nil, fmt.Errorf("readwise: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, readwiseBaseURL+"/highlights/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("readwise: build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+r.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("readwise: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("readwise: unexpected status %d", resp.StatusCode)
	}

	var result readwiseCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("readwise: decode response: %w", err)
	}

	results := make([]ExportResult, len(highlights))
	for i, h := range highlights {
		results[i].HighlightID = h.ID
		if i < len(result.Highlights) {
			results[i].ExternalID = strconv.FormatInt(result.Highlights[i].ID, 10)
		} else {
			results[i].Err = fmt.Errorf("readwise: no ID returned for highlight %s", h.ID)
		}
	}
	return results, nil
}

func (r *ReadwiseBackend) Delete(ctx context.Context, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		readwiseBaseURL+"/highlights/"+externalID+"/", nil)
	if err != nil {
		return fmt.Errorf("readwise: build delete request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+r.apiToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("readwise: delete request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("readwise: delete unexpected status %d", resp.StatusCode)
	}
	return nil
}
