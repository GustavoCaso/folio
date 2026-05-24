package handlers

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/converter"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/parser/client"
	"github.com/GustavoCaso/folio/ui/internal/repository"
	"github.com/templui/templui/utils"
)

//go:embed static/js/reader.js static/js/documents.js static/tailwind/output.css static/css/reader.css
var staticFS embed.FS

type Handlers struct {
	store     repository.Store
	hub       *hub.Hub
	parser    client.Client
	converter *converter.Runner
	dataDir   string
	backends  []export.Backend
}

func (h *Handlers) backendByName(name string) export.Backend {
	for _, b := range h.backends {
		if b.Name() == name {
			return b
		}
	}
	return nil
}

func Register(store repository.Store, h *hub.Hub, pc client.Client, dataDir string, logger *slog.Logger, backends []export.Backend) (*http.ServeMux, error) {
	if logger == nil {
		return nil, errors.New("handlers.Register: logger is required")
	}

	if h == nil {
		return nil, errors.New("handlers.Register: hub.Hub is required")
	}

	if store == nil {
		return nil, errors.New("handlers.Register: store is required")
	}

	var runner *converter.Runner
	if pc != nil {
		runner = converter.New(store, h, pc, dataDir, logger)
	}

	hs := &Handlers{store: store, hub: h, parser: pc, converter: runner, dataDir: dataDir, backends: backends}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", hs.ListDocuments)
	mux.HandleFunc("GET /health/parser", hs.ParserHealth)
	mux.HandleFunc("POST /documents", hs.UploadDocument)
	mux.HandleFunc("POST /documents/import", hs.ImportDocument)
	mux.HandleFunc("POST /documents/{id}/cancel", hs.CancelDocument)
	mux.HandleFunc("POST /documents/{id}/retry", hs.RetryDocument)
	mux.HandleFunc("DELETE /documents/{id}", hs.DeleteDocument)
	mux.HandleFunc("GET /read/{jobID}", hs.ReadDocument)
	mux.HandleFunc("GET /jobs/{jobID}/watch", hs.WatchJob)
	mux.HandleFunc("POST /highlights", hs.CreateHighlight)
	mux.HandleFunc("DELETE /highlights/{id}", hs.DeleteHighlight)
	mux.HandleFunc("POST /read/{jobID}/progress", hs.UpdateReadingProgress)
	mux.HandleFunc("GET /exports", hs.ListExports)

	isDev := os.Getenv("ENV") != "production"
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	utils.SetupScriptRoutes(mux, isDev)

	return mux, nil
}

func (h *Handlers) ParserHealth(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status := "unhealthy"
	if h.parser != nil && h.parser.Health(ctx) {
		status = "healthy"
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if err := json.NewEncoder(w).Encode(map[string]string{"status": status}); err != nil {
		log.Error("ParserHealth: encode response", "err", err)
	}
}
