package db

import (
	"database/sql"
	"embed"

	"github.com/GustavoCaso/folio/ui/internal/repository"
	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// db is the SQLite storage layer. It owns all SQL operations and satisfies repository.Store.
type db struct {
	conn *sql.DB
}

func New(path string) (repository.Store, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite is single-writer

	if err := runMigrations(sqlDB); err != nil {
		return nil, err
	}

	return &db{conn: sqlDB}, nil
}

func (d *db) Close() error {
	return d.conn.Close()
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
