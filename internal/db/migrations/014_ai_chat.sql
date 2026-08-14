-- Reading-assistant chat: one session per article, multi-turn messages.

CREATE TABLE IF NOT EXISTS ai_chat_sessions (
  id            TEXT PRIMARY KEY,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  article_id    TEXT NOT NULL DEFAULT '',
  collection_id TEXT NOT NULL DEFAULT '',
  locale        TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_chat_sessions_article
  ON ai_chat_sessions(article_id)
  WHERE article_id IS NOT NULL AND article_id != '';

CREATE TABLE IF NOT EXISTS ai_chat_messages (
  id              TEXT PRIMARY KEY,
  session_id      TEXT NOT NULL,
  role            TEXT NOT NULL,
  content         TEXT NOT NULL DEFAULT '',
  citations_json  TEXT NOT NULL DEFAULT '[]',
  created_at      TEXT NOT NULL,
  FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_session
  ON ai_chat_messages(session_id, created_at);
