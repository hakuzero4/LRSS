-- Cache for LLM feature results (summarize / translate / ask / classify / suggest).
-- Keyed by article (optional) + feature + model + content fingerprint.

CREATE TABLE IF NOT EXISTS llm_feature_cache (
  cache_key    TEXT PRIMARY KEY,
  article_id   TEXT,
  feature      TEXT NOT NULL,
  model        TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  result_md    TEXT NOT NULL,
  meta_json    TEXT,
  created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_llm_cache_article ON llm_feature_cache(article_id, feature);
