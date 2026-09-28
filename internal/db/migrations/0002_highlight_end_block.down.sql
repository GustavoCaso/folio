ALTER TABLE highlights DROP COLUMN end_block_id;
ALTER TABLE highlights RENAME COLUMN start_block_id TO block_id;
