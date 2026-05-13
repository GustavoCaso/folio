package readwise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
	logger   *slog.Logger
}

// New creates a Readwise export backend from cfg.
func New(cfg Config, logger *slog.Logger) (export.Backend, error) {
	if logger == nil {
		return nil, errors.New("readwise.New: logger is required")
	}

	base := cfg.BaseURL
	if base == "" {
		base = readwiseBaseURL
	}
	return &ReadwiseBackend{
		apiToken: cfg.APIToken,
		baseURL:  base,
		client:   &http.Client{Timeout: cfg.Timeout},
		logger:   logger,
	}, nil
}

func (r *ReadwiseBackend) Name() string { return "readwise" }

func (r *ReadwiseBackend) Export(ctx context.Context, exports []*export.ExportRecord) error {
	inputs := make([]ReadwiseHighlightInput, len(exports))
	for i, rec := range exports {
		inputs[i] = ReadwiseHighlightInput{
			Text:     rec.HighlightText,
			Title:    rec.Title,
			Category: "books",
			Note:     rec.Note,
		}
	}

	body, err := json.Marshal(ReadwiseCreateRequest{Highlights: inputs})
	if err != nil {
		return fmt.Errorf("readwise: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/highlights/", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("readwise: build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+r.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("readwise: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("readwise: unexpected status %d", resp.StatusCode)
	}

	// Response is a plain JSON array of created highlight objects.
	var created []ReadwiseHighlightResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return fmt.Errorf("readwise: decode response: %w", err)
	}

	if len(created) == 0 {
		return fmt.Errorf("readwise: empty response when creating highlights")
	}

	modifiedHighlights := created[0].ModifiedHighlights
	createdHighlights := len(modifiedHighlights)

	for i, export := range exports {
		if i < createdHighlights {
			export.ExternalID = strconv.FormatInt(modifiedHighlights[i], 10)
		} else {
			export.Err = fmt.Errorf("readwise: no ID returned for highlight_export %s", export.ExportID)
		}
	}

	// Add tags via a separate per-highlight endpoint (Readwise has no tag field on creation).
	for i, rec := range exports {
		if rec.Tag != "" && exports[i].ExternalID != "" {
			r.addTag(ctx, exports[i].ExternalID, rec.Tag)
		}
	}

	return nil
}

func (r *ReadwiseBackend) addTag(ctx context.Context, externalID, tag string) {
	body, err := json.Marshal(ReadwiseTagRequest{Name: tag})
	if err != nil {
		r.logger.Warn("readwise: marshal tag request", "external_id", externalID, "err", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.baseURL+"/highlights/"+externalID+"/tags/", bytes.NewReader(body))
	if err != nil {
		r.logger.Warn("readwise: build tag request", "external_id", externalID, "err", err)
		return
	}
	req.Header.Set("Authorization", "Token "+r.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Warn("readwise: add tag failed", "external_id", externalID, "tag", tag, "err", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.logger.Warn("readwise: add tag unexpected status", "external_id", externalID, "tag", tag, "status", resp.StatusCode)
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
