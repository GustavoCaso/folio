CREATE TABLE IF NOT EXISTS highlight_exports (
    id           TEXT PRIMARY KEY,
    highlight_id TEXT NOT NULL REFERENCES highlights(id) ON DELETE CASCADE,
    backend_name TEXT NOT NULL,
    external_id  TEXT, 
    status       TEXT NOT NULL DEFAULT 'PENDING',
    error        TEXT,
    exported_at  TEXT, 
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
