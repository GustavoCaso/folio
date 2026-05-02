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

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
	"github.com/templui/templui/utils"
)

//go:embed static/js/highlight.js static/js/documents.js static/tailwind/output.css static/css/reader.css
var staticFS embed.FS

// ParserClient is the interface satisfied by *parserclient.Client (and fakes in tests).
type ParserClient interface {
	Convert(ctx context.Context, jobID, requestID, filename string, pdfBytes []byte, h *hub.Hub) (parserclient.ConversionResult, error)
	Health(ctx context.Context) bool
}

type Handlers struct {
	store   *db.Store
	hub     *hub.Hub
	parser  ParserClient
	dataDir string
	// logger is used only by background goroutines (e.g. runConversion) that
	// outlive the originating request and therefore can't use
	// logging.LoggerFrom. Request-scoped code should pull the logger — with
	// request_id attached by middleware — from r.Context() instead.
	logger *slog.Logger
}

func Register(store *db.Store, h *hub.Hub, pc ParserClient, dataDir string, logger *slog.Logger) (*http.ServeMux, error) {
	if logger == nil {
		return nil, errors.New("handlers.Register: logger is required")
	}

	hs := &Handlers{store: store, hub: h, parser: pc, dataDir: dataDir, logger: logger}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", hs.ListDocuments)
	mux.HandleFunc("GET /health/parser", hs.ParserHealth)
	mux.HandleFunc("POST /documents", hs.UploadDocument)
	mux.HandleFunc("POST /documents/{id}/retry", hs.RetryDocument)
	mux.HandleFunc("POST /documents/{id}/delete", hs.DeleteDocument)
	mux.HandleFunc("GET /read/{jobID}", hs.ReadDocument)
	mux.HandleFunc("GET /jobs/{jobID}/watch", hs.WatchJob)
	mux.HandleFunc("POST /highlights", hs.CreateHighlight)
	mux.HandleFunc("DELETE /highlights/{id}", hs.DeleteHighlight)

	isDev := os.Getenv("ENV") != "production"
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	utils.SetupScriptRoutes(mux, isDev)

	return mux, nil
}

func (h *Handlers) ParserHealth(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("ParserHealth: encode response", "err", err)
	}
}
