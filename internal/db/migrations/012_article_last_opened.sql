-- Recently-read cursor; NULL means never opened (or pruned from the recent list).
ALTER TABLE articles ADD COLUMN last_opened_at TEXT;
CREATE INDEX IF NOT EXISTS idx_articles_last_opened
  ON articles(last_opened_at DESC) WHERE last_opened_at IS NOT NULL;
