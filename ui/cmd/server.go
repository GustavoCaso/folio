package cmd

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

func Execute() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", envOr("DB_PATH", "/data/folio.db"), "SQLite DB path")
	parserAddr := flag.String("parser", envOr("PARSER_GRPC_ADDR", "localhost:50051"), "Parser gRPC address")
	dataDir := flag.String("data", envOr("DATA_DIR", "/data"), "Directory for Markdown files")
	flag.Parse()

	store, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	h := hub.New()

	pc, err := parserclient.New(*parserAddr)
	if err != nil {
		log.Fatalf("grpc: %v", err)
	}
	defer pc.Close()

	mux := handlers.Register(store, h, pc, *dataDir)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Printf("Listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
