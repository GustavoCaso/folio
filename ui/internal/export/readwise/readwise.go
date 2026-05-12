package readwise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
)

const readwiseBaseURL = "https://readwise.io/api/v2"

// Config holds configuration for the Readwise export backend.
type Config struct {
	APIToken string
	Timeout  time.Duration
	// BaseURL overrides the Readwise API base URL. Leave empty for production.
	BaseURL string
}

type ReadwiseBackend struct {
	apiToken string
	baseURL  string
	client   *http.Client
}

// New creates a Readwise export backend from cfg.
func New(cfg Config) export.Backend {
	base := cfg.BaseURL
	if base == "" {
		base = readwiseBaseURL
	}
	return &ReadwiseBackend{
		apiToken: cfg.APIToken,
		baseURL:  base,
		client:   &http.Client{Timeout: cfg.Timeout},
	}
}

func (r *ReadwiseBackend) Name() string { return "readwise" }

func (r *ReadwiseBackend) Export(ctx context.Context, records []db.ExportRecord) ([]export.ExportResult, error) {
	inputs := make([]ReadwiseHighlightInput, len(records))
	for i, rec := range records {
		inputs[i] = ReadwiseHighlightInput{
			Text:     rec.HighlightText,
			Title:    rec.JobFilename,
			Category: "books",
			Note:     rec.HighlightNote,
		}
	}

	body, err := json.Marshal(ReadwiseCreateRequest{Highlights: inputs})
	if err != nil {
		return nil, fmt.Errorf("readwise: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/highlights/", bytes.NewReader(body))
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

	// Response is a plain JSON array of created highlight objects.
	var created []ReadwiseHighlightResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("readwise: decode response: %w", err)
	}

	modifiedHighlights := created[0].ModifiedHighlights

	results := make([]export.ExportResult, len(records))
	for i, rec := range records {
		results[i].ExportID = rec.ID
		if i < len(modifiedHighlights) {
			results[i].ExternalID = strconv.FormatInt(modifiedHighlights[i], 10)
		} else {
			results[i].Err = fmt.Errorf("readwise: no ID returned for highlight %s", rec.HighlightID)
		}
	}

	// Add tags via a separate per-highlight endpoint (Readwise has no tag field on creation).
	for i, rec := range records {
		if rec.HighlightTag != "" && results[i].ExternalID != "" {
			r.addTag(ctx, results[i].ExternalID, rec.HighlightTag)
		}
	}

	return results, nil
}

func (r *ReadwiseBackend) addTag(ctx context.Context, externalID, tag string) {
	body, err := json.Marshal(ReadwiseTagRequest{Name: tag})
	if err != nil {
		slog.Default().Warn("readwise: marshal tag request", "external_id", externalID, "err", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.baseURL+"/highlights/"+externalID+"/tags/", bytes.NewReader(body))
	if err != nil {
		slog.Default().Warn("readwise: build tag request", "external_id", externalID, "err", err)
		return
	}
	req.Header.Set("Authorization", "Token "+r.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		slog.Default().Warn("readwise: add tag failed", "external_id", externalID, "tag", tag, "err", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Default().Warn("readwise: add tag unexpected status", "external_id", externalID, "tag", tag, "status", resp.StatusCode)
	}
}

func (r *ReadwiseBackend) Delete(ctx context.Context, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		r.baseURL+"/highlights/"+externalID+"/", nil)
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
