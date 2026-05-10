ALTER TABLE jobs DROP COLUMN pages_done;
ALTER TABLE jobs DROP COLUMN pages_total;
ALTER TABLE jobs ADD COLUMN reading_progress TEXT NOT NULL DEFAULT '';
