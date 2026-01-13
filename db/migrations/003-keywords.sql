-- Add keywords column to bookmarks
ALTER TABLE bookmarks ADD COLUMN keywords TEXT;

-- Record execution of this migration
INSERT OR IGNORE INTO migrations (migration_number, migration_name)
VALUES (003, '003-keywords');
