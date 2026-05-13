package cmd

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/export"
	"github.com/GustavoCaso/folio/ui/internal/export/readwise"
	"github.com/GustavoCaso/folio/ui/internal/handlers"
	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	parserclient "github.com/GustavoCaso/folio/ui/internal/parser/client"
	"github.com/GustavoCaso/folio/ui/internal/repository"
)

func Execute() {
	execute(func(path string) (repository.Store, error) { return db.New(path) })
}

func execute(openStore func(path string) (repository.Store, error)) {
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

	store, err := openStore(*dbPath)
	if err != nil {
		logger.Error("db open failed", logging.Err(err), "db_path", *dbPath)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("db close failed", logging.Err(err))
		}
	}()

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
	defer func() {
		if err := pc.Close(); err != nil {
			logger.Error("parser client close failed", logging.Err(err))
		}
	}()

	// Build export backends from environment variables.
	var backends []export.Backend
	if token := os.Getenv("READWISE_API_TOKEN"); token != "" {
		timeoutStr := envOr("READWISE_TIMEOUT", "30s")
		readwiseTimeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warn("invalid READWISE_TIMEOUT, using default 30s", "value", timeoutStr)
			readwiseTimeout = 30 * time.Second
		}
		backend, err := readwise.New(readwise.Config{
			APIToken: token,
			Timeout:  readwiseTimeout,
		}, logger.With("component", "export.readwise"))

		if err != nil {
			logger.Error("readwise backend init failed", logging.Err(err))
			os.Exit(1)
		}
		backends = append(backends, backend)
		logger.Info("readwise export backend enabled", "timeout", readwiseTimeout)
	}

	// Start the export worker if any backends are configured.
	if len(backends) > 0 {
		intervalStr := envOr("EXPORT_INTERVAL", "1m")
		exportInterval, err := time.ParseDuration(intervalStr)
		if err != nil {
			logger.Warn("invalid EXPORT_INTERVAL, using default 1m", "value", intervalStr)
			exportInterval = time.Minute
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		worker, err := export.NewWorker(store, backends, exportInterval, logger.With("component", "export.worker"))
		if err != nil {
			logger.Error("export init failed", logging.Err(err))
			os.Exit(1)
		}
		go worker.Run(ctx)
		logger.Info("export worker started", "interval", exportInterval)
	}

	mux, err := handlers.Register(store, h, pc, *dataDir, logger.With("component", "handlers"), backends)
	if err != nil {
		logger.Error("handlers register failed", logging.Err(err))
		os.Exit(1)
	}

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
