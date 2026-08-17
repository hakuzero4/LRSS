-- Smart-filter 精选: articles kept manually or by the filter worker.

CREATE TABLE IF NOT EXISTS article_keeps (
  article_id  TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  reason      TEXT NOT NULL DEFAULT '',
  confidence  REAL NOT NULL DEFAULT 0,
  topics      TEXT NOT NULL DEFAULT '[]',
  source      TEXT NOT NULL DEFAULT 'filter',
  created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_article_keeps_created ON article_keeps(created_at DESC);
