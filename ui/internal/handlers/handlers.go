package handlers

import (
	"embed"
	"errors"
	"log/slog"
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

//go:embed static/highlight.js
var staticFS embed.FS

type Handlers struct {
	store   *db.Store
	hub     *hub.Hub
	parser  *parserclient.Client
	dataDir string
	// logger is used only by background goroutines (e.g. runConversion) that
	// outlive the originating request and therefore can't use
	// logging.LoggerFrom. Request-scoped code should pull the logger — with
	// request_id attached by middleware — from r.Context() instead.
	logger *slog.Logger
}

func Register(store *db.Store, h *hub.Hub, pc *parserclient.Client, dataDir string, logger *slog.Logger) (*http.ServeMux, error) {
	if logger == nil {
		return nil, errors.New("handlers.Register: logger is required")
	}

	hs := &Handlers{store: store, hub: h, parser: pc, dataDir: dataDir, logger: logger}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", hs.ListDocuments)
	mux.HandleFunc("POST /documents", hs.UploadDocument)
	mux.HandleFunc("POST /documents/{id}/delete", hs.DeleteDocument)
	mux.HandleFunc("GET /read/{jobID}", hs.ReadDocument)
	mux.HandleFunc("GET /jobs/{jobID}/watch", hs.WatchJob)
	mux.HandleFunc("POST /highlights", hs.CreateHighlight)
	mux.HandleFunc("DELETE /highlights/{id}", hs.DeleteHighlight)

	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))

	return mux, nil
}
