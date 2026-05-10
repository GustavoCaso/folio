CREATE TABLE IF NOT EXISTS highlight_exports (
    id           TEXT PRIMARY KEY,
    highlight_id TEXT NOT NULL REFERENCES highlights(id) ON DELETE CASCADE,
    backend_name TEXT NOT NULL,
    external_id  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'exported',
    error        TEXT NOT NULL DEFAULT '',
    exported_at  TEXT NOT NULL
);
