ALTER TABLE highlights RENAME COLUMN block_id TO start_block_id;
ALTER TABLE highlights ADD COLUMN end_block_id TEXT NOT NULL DEFAULT '';
