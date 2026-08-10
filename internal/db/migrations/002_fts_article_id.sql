-- Rebuild FTS with article_id for reliable joins (TEXT primary keys).
DROP TABLE IF EXISTS articles_fts;

CREATE VIRTUAL TABLE articles_fts USING fts5(
  article_id UNINDEXED,
  title,
  summary,
  content_text,
  tokenize = 'unicode61'
);
