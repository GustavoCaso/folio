package db

import (
	"database/sql"
	"embed"
	"errors"

	"github.com/GustavoCaso/folio/ui/internal/repository"
	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB is the raw SQLite storage layer. It manages the connection and migrations.
type DB struct {
	db *sql.DB
}

func New(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite is single-writer

	if err := runMigrations(sqlDB); err != nil {
		return nil, err
	}

	return &DB{db: sqlDB}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// Repository wraps a DB and implements all repository interfaces.
type Repository struct {
	db *DB
}

var _ repository.Store = (*Repository)(nil)

func NewRepository(d *DB) (*Repository, error) {
	if d == nil {
		return nil, errors.New("db.NewRepository: db is required")
	}
	return &Repository{db: d}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func runMigrations(db *sql.DB) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}
	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
