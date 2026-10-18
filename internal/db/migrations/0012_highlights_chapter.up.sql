ALTER TABLE highlights ADD COLUMN chapter INTEGER NOT NULL DEFAULT 0;

-- Backfill from the ch{N}- prefix on start_block_id (ADR 0001).
UPDATE highlights
SET chapter = CAST(substr(start_block_id, 3, instr(start_block_id, '-') - 3) AS INTEGER)
WHERE start_block_id GLOB 'ch[0-9]*-*';

CREATE INDEX IF NOT EXISTS highlights_job_chapter ON highlights (job_id, chapter);
