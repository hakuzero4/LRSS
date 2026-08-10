-- LRSS schema v1
-- ISO-8601 TEXT timestamps; IDs are application-generated (ULID/UUID strings).

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version     INTEGER PRIMARY KEY,
  applied_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS folders (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  parent_id   TEXT REFERENCES folders(id) ON DELETE SET NULL,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS feeds (
  id              TEXT PRIMARY KEY,
  folder_id       TEXT REFERENCES folders(id) ON DELETE SET NULL,
  title           TEXT NOT NULL,
  site_url        TEXT,
  feed_url        TEXT NOT NULL UNIQUE,
  favicon_url     TEXT,
  etag            TEXT,
  last_modified   TEXT,
  last_fetched_at TEXT,
  last_error      TEXT,
  is_paused       INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_feeds_folder ON feeds(folder_id);

CREATE TABLE IF NOT EXISTS articles (
  id            TEXT PRIMARY KEY,
  feed_id       TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid          TEXT,
  url           TEXT NOT NULL,
  title         TEXT NOT NULL,
  author        TEXT,
  summary       TEXT,
  content_html  TEXT,
  content_text  TEXT,
  image_url     TEXT,
  published_at  TEXT,
  fetched_at    TEXT NOT NULL,
  is_read       INTEGER NOT NULL DEFAULT 0,
  is_starred    INTEGER NOT NULL DEFAULT 0,
  UNIQUE (feed_id, guid)
);

CREATE INDEX IF NOT EXISTS idx_articles_feed_pub ON articles(feed_id, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_read_pub ON articles(is_read, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_starred ON articles(is_starred) WHERE is_starred = 1;
CREATE INDEX IF NOT EXISTS idx_articles_url ON articles(url);

-- Application settings (JSON values)
CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Vector rows: embedding BLOB is FLOAT32 packed; NULL while pending/error.
-- sqlite-vector is initialized at runtime when embedding is configured (S2).
CREATE TABLE IF NOT EXISTS article_embeddings (
  article_id    TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  model         TEXT NOT NULL DEFAULT '',
  dimensions    INTEGER NOT NULL DEFAULT 0,
  embedding     BLOB,
  content_hash  TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'ready', 'error', 'skipped')),
  error         TEXT,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_embeddings_status ON article_embeddings(status);

CREATE TABLE IF NOT EXISTS jobs (
  id          TEXT PRIMARY KEY,
  kind        TEXT NOT NULL,
  payload     TEXT,
  status      TEXT NOT NULL DEFAULT 'queued'
              CHECK (status IN ('queued', 'running', 'done', 'error', 'cancelled')),
  progress    REAL NOT NULL DEFAULT 0,
  error       TEXT,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_kind_status ON jobs(kind, status);

-- Standalone FTS5 index; application keeps it in sync (no content= triggers).
CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
  title,
  summary,
  content_text,
  tokenize = 'unicode61'
);
