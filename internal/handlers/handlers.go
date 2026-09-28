package handlers

import (
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/GustavoCaso/folio/ui/internal/converter"
	"github.com/GustavoCaso/folio/ui/internal/converter/parser"
	"github.com/GustavoCaso/folio/ui/internal/domain"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/repository"
	"github.com/templui/templui/utils"
)

//go:embed static/js/reader.js static/js/documents.js static/tailwind/output.css static/css/reader.css
var staticFS embed.FS

type Handlers struct {
	store     repository.Store
	hub       *hub.Hub
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

func Register(store repository.Store, h *hub.Hub, dataDir string, logger *slog.Logger, backends []export.Backend) (*http.ServeMux, error) {
	parsers := map[domain.JobFormat]parser.Parser{
		domain.EpubFormat: parser.NewEPUB(store, h, dataDir),
	}
	return register(store, h, dataDir, logger, backends, parsers)
}

func register(store repository.Store, h *hub.Hub, dataDir string, logger *slog.Logger, backends []export.Backend, parsers map[domain.JobFormat]parser.Parser) (*http.ServeMux, error) {
	if logger == nil {
		return nil, errors.New("handlers.Register: logger is required")
	}

	if h == nil {
		return nil, errors.New("handlers.Register: hub.Hub is required")
	}

	if store == nil {
		return nil, errors.New("handlers.Register: store is required")
	}

	runner := converter.New(store, h, parsers, logger)

	hs := &Handlers{store: store, hub: h, converter: runner, dataDir: dataDir, backends: backends}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", hs.ListDocuments)
	mux.HandleFunc("POST /documents", hs.UploadDocument)
	mux.HandleFunc("POST /documents/{id}/cancel", hs.CancelDocument)
	mux.HandleFunc("POST /documents/{id}/retry", hs.RetryDocument)
	mux.HandleFunc("DELETE /documents/{id}", hs.DeleteDocument)
	mux.HandleFunc("GET /documents/{id}/edit", hs.EditDocumentForm)
	mux.HandleFunc("POST /documents/{id}/edit", hs.EditDocument)
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
