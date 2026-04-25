PRAGMA foreign_keys=off;

ALTER TABLE jobs RENAME TO old_jobs;

CREATE TABLE jobs
(
    id          TEXT PRIMARY KEY,
    filename    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'PENDING',
    pages_done  INTEGER NOT NULL DEFAULT 0,
    pages_total INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    output_path TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT ''
);

INSERT INTO jobs SELECT * FROM old_jobs;

PRAGMA foreign_keys=on;
