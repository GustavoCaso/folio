package db

import (
	"context"
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

// sqlExecutor is satisfied by both *sql.DB and *sql.Tx.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// db is the SQLite storage layer. It owns all SQL operations and satisfies repository.Store.
type db struct {
	conn    sqlExecutor // *sql.DB normally, *sql.Tx inside a transaction
	rawConn *sql.DB     // nil inside a transaction
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

	return &db{conn: sqlDB, rawConn: sqlDB}, nil
}

func (d *db) Close() error {
	if d.rawConn == nil {
		return nil
	}
	return d.rawConn.Close()
}

func (d *db) WithTx(ctx context.Context, fn func(ctx context.Context, store repository.Store) error) error {
	if d.rawConn == nil {
		return errors.New("db.WithTx: cannot start a transaction inside a transaction")
	}
	tx, err := d.rawConn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txDB := &db{conn: tx, rawConn: nil}
	if err := fn(ctx, txDB); err != nil {
		txError := tx.Rollback()
		return errors.Join([]error{txError, err}...)
	}
	return tx.Commit()
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
