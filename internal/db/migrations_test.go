package db

import (
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func newMigrator(t *testing.T, conn *sql.DB) *migrate.Migrate {
	t.Helper()
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := migratesqlite.WithInstance(conn, &migratesqlite.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Migrations 0012 and 0013 must carry existing rows over: highlight chapters
// are backfilled from start_block_id, and jobs move to documents intact.
func TestMigrations_ChapterBackfillAndDocumentsRename(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	defer func() { _ = conn.Close() }()

	m := newMigrator(t, conn)
	if err := m.Migrate(11); err != nil {
		t.Fatal(err)
	}

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO jobs (id, filename, format, created_at, updated_at) VALUES ('j1', 'old.pdf', 'pdf', 'now', 'now')`)
	mustExec(`INSERT INTO highlights (id, job_id, start_block_id, start_pos, end_pos, text, created_at)
		VALUES ('h0', 'j1', 'ch0-p-1', 0, 1, 'a', 'now'),
		       ('h12', 'j1', 'ch12-h2-3', 0, 1, 'b', 'now'),
		       ('hlegacy', 'j1', 'paragraph-3', 0, 1, 'c', 'now')`)

	if err := m.Up(); err != nil {
		t.Fatal(err)
	}

	want := map[string]int{"h0": 0, "h12": 12, "hlegacy": 0}
	for id, chapter := range want {
		var got int
		if err := conn.QueryRow(`SELECT chapter FROM highlights WHERE id = ?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != chapter {
			t.Errorf("highlight %s: expected chapter %d, got %d", id, chapter, got)
		}
	}

	var format string
	if err := conn.QueryRow(`SELECT format FROM documents WHERE id = 'j1'`).Scan(&format); err != nil {
		t.Fatal(err)
	}
	if format != "pdf" {
		t.Errorf("existing row format should be preserved, got %q", format)
	}

	mustExec(`INSERT INTO documents (id, filename, created_at, updated_at) VALUES ('j2', 'new.epub', 'now', 'now')`)
	if err := conn.QueryRow(`SELECT format FROM documents WHERE id = 'j2'`).Scan(&format); err != nil {
		t.Fatal(err)
	}
	if format != "epub" {
		t.Errorf("expected format to default to epub, got %q", format)
	}

	var table, from string
	if err := conn.QueryRow(`SELECT "table", "from" FROM pragma_foreign_key_list('highlights')`).Scan(&table, &from); err != nil {
		t.Fatal(err)
	}
	if table != "documents" || from != "document_id" {
		t.Errorf("expected highlights.document_id to reference documents, got %s -> %s", from, table)
	}

	var documentID string
	if err := conn.QueryRow(`SELECT document_id FROM highlights WHERE id = 'h0'`).Scan(&documentID); err != nil {
		t.Fatal(err)
	}
	if documentID != "j1" {
		t.Errorf("expected highlight to keep its document, got %q", documentID)
	}

	var index string
	if err := conn.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'highlights_document_chapter'`).Scan(&index); err != nil {
		t.Fatalf("expected highlights_document_chapter index after rebuild: %v", err)
	}

	if err := m.Migrate(11); err != nil {
		t.Fatalf("down migrations failed: %v", err)
	}
	if err := conn.QueryRow(`SELECT format FROM jobs WHERE id = 'j2'`).Scan(&format); err != nil {
		t.Fatal(err)
	}
	var jobID string
	if err := conn.QueryRow(`SELECT job_id FROM highlights WHERE id = 'h0'`).Scan(&jobID); err != nil {
		t.Fatalf("down migration should restore highlights.job_id: %v", err)
	}
}
