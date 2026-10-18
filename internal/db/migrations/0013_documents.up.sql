-- Rename jobs to documents and default format to 'epub'. SQLite cannot change
-- a column default in place, so both tables are rebuilt. highlights is rebuilt
-- too: its job_id foreign key has pointed at the dropped old_jobs table since
-- migration 0004, so it is recreated as document_id referencing documents(id).

CREATE TABLE documents
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
    format           TEXT NOT NULL DEFAULT 'epub',

    CONSTRAINT filename_unique UNIQUE (filename)
);

INSERT INTO documents (id, filename, status, error, output_path, created_at, updated_at, request_id, content, retry_count, reading_progress, title, author, cover, source_url, tags, format)
SELECT id, filename, status, error, output_path, created_at, updated_at, request_id, content, retry_count, reading_progress, title, author, cover, source_url, tags, format FROM jobs;

CREATE TABLE highlights_new
(
    id             TEXT PRIMARY KEY,
    document_id    TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
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

INSERT INTO highlights_new (id, document_id, start_block_id, end_block_id, chapter, start_pos, end_pos, text, tag, note, created_at)
SELECT id, job_id, start_block_id, end_block_id, chapter, start_pos, end_pos, text, tag, note, created_at FROM highlights;

DROP TABLE highlights;
DROP TABLE jobs;

ALTER TABLE highlights_new RENAME TO highlights;

CREATE INDEX IF NOT EXISTS highlights_document_chapter ON highlights (document_id, chapter);
