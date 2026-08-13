-- Per-folder article list layout. list = existing rows; cards = image grid.
ALTER TABLE folders ADD COLUMN display_mode TEXT NOT NULL DEFAULT 'list';
