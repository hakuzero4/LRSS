-- Stored AI briefings (智能汇报): generated summaries over recent unread articles.

CREATE TABLE IF NOT EXISTS ai_briefings (
  id            TEXT PRIMARY KEY,
  created_at    TEXT NOT NULL,
  status        TEXT NOT NULL,
  locale        TEXT NOT NULL,
  model         TEXT,
  overview      TEXT NOT NULL DEFAULT '',
  error         TEXT,
  article_count INTEGER NOT NULL DEFAULT 0,
  omitted_count INTEGER NOT NULL DEFAULT 0,
  is_read       INTEGER NOT NULL DEFAULT 0,
  is_starred    INTEGER NOT NULL DEFAULT 0,
  payload_json  TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_ai_briefings_created ON ai_briefings(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_briefings_unread ON ai_briefings(is_read) WHERE is_read = 0;
