package handlers

import (
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

type Handlers struct {
	store   *db.Store
	hub     *hub.Hub
	parser  *parserclient.Client
	dataDir string
}

func Register(store *db.Store, h *hub.Hub, pc *parserclient.Client, dataDir string) *http.ServeMux {
	hs := &Handlers{store: store, hub: h, parser: pc, dataDir: dataDir}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", hs.ListDocuments)
	mux.HandleFunc("POST /documents", hs.UploadDocument)
	mux.HandleFunc("GET /read/{jobID}", hs.ReadDocument)
	mux.HandleFunc("GET /jobs/{jobID}/watch", hs.WatchJob)
	mux.HandleFunc("POST /highlights", hs.CreateHighlight)
	mux.HandleFunc("DELETE /highlights/{id}", hs.DeleteHighlight)

	return mux
}
