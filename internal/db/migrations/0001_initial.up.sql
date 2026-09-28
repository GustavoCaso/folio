CREATE TABLE IF NOT EXISTS jobs (
    id          TEXT PRIMARY KEY,
    filename    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'PENDING',
    pages_done  INTEGER NOT NULL DEFAULT 0,
    pages_total INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    output_path TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- block_id anchors a highlight to a specific block-level element
-- (heading slug or paragraph index) rather than a raw document offset,
-- so highlights survive minor re-renders.
-- Renamed to start_block_id in migration 0002 (multi-block highlights).
CREATE TABLE IF NOT EXISTS highlights (
    id          TEXT PRIMARY KEY,
    job_id      TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    block_id    TEXT NOT NULL,
    start_pos   INTEGER NOT NULL,
    end_pos     INTEGER NOT NULL,
    text        TEXT NOT NULL,
    tag         TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
