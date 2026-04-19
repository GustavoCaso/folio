package cmd

import (
	"flag"
	"net/http"
	"os"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
)

func Execute() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", envOr("DB_PATH", "/data/folio.db"), "SQLite DB path")
	parserAddr := flag.String("parser", envOr("PARSER_GRPC_ADDR", "localhost:50051"), "Parser gRPC address")
	dataDir := flag.String("data", envOr("DATA_DIR", "/data"), "Directory for Markdown files")
	logLevel := flag.String("log-level", envOr("LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	flag.Parse()

	logger := logging.Init(*logLevel).With("service", "ui")
	logger.Info("starting",
		"addr", *addr,
		"db_path", *dbPath,
		"parser_addr", *parserAddr,
		"data_dir", *dataDir,
		"log_level", *logLevel,
	)

	store, err := db.New(*dbPath)
	if err != nil {
		logger.Error("db open failed", logging.Err(err), "db_path", *dbPath)
		os.Exit(1)
	}
	defer store.Close()

	h, err := hub.New(logger.With("component", "hub"))
	if err != nil {
		logger.Error("hub init failed", logging.Err(err))
		os.Exit(1)
	}

	pc, err := parserclient.New(*parserAddr, logger.With("component", "parser.client"))
	if err != nil {
		logger.Error("grpc dial failed", logging.Err(err), "parser_addr", *parserAddr)
		os.Exit(1)
	}
	defer pc.Close()

	mux, err := handlers.Register(store, h, pc, *dataDir, logger.With("component", "handlers"))
	if err != nil {
		logger.Error("handlers register failed", logging.Err(err))
		os.Exit(1)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	srv := logging.Middleware(logger)(mux)

	logger.Info("listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		logger.Error("http server stopped", logging.Err(err))
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
