CREATE TABLE jobs
(
    id               TEXT PRIMARY KEY,
    filename         TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'PENDING',
    error            TEXT NOT NULL DEFAULT '',
    output_path      TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    request_id       TEXT NOT NULL DEFAULT '',
    content          BLOB,
    retry_count      INTEGER NOT NULL DEFAULT 0,
    reading_progress TEXT NOT NULL DEFAULT '',
    title            TEXT NOT NULL DEFAULT '',
    author           TEXT NOT NULL DEFAULT '',
    cover            BLOB,
    source_url       TEXT NOT NULL DEFAULT '',
    tags             TEXT NOT NULL DEFAULT '',
    format           TEXT NOT NULL DEFAULT 'pdf',

    CONSTRAINT filename_unique UNIQUE (filename)
);

INSERT INTO jobs (id, filename, status, error, output_path, created_at, updated_at, request_id, content, retry_count, reading_progress, title, author, cover, source_url, tags, format)
SELECT id, filename, status, error, output_path, created_at, updated_at, request_id, content, retry_count, reading_progress, title, author, cover, source_url, tags, format FROM documents;

CREATE TABLE highlights_new
(
    id             TEXT PRIMARY KEY,
    job_id         TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    start_block_id TEXT NOT NULL,
    end_block_id   TEXT NOT NULL DEFAULT '',
    chapter        INTEGER NOT NULL DEFAULT 0,
    start_pos      INTEGER NOT NULL,
    end_pos        INTEGER NOT NULL,
    text           TEXT NOT NULL,
    tag            TEXT NOT NULL DEFAULT '',
    note           TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

INSERT INTO highlights_new (id, job_id, start_block_id, end_block_id, chapter, start_pos, end_pos, text, tag, note, created_at)
SELECT id, document_id, start_block_id, end_block_id, chapter, start_pos, end_pos, text, tag, note, created_at FROM highlights;

DROP TABLE highlights;
DROP TABLE documents;

ALTER TABLE highlights_new RENAME TO highlights;

CREATE INDEX IF NOT EXISTS highlights_job_chapter ON highlights (job_id, chapter);
