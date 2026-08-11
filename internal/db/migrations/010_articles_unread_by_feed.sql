-- Speeds up unread counts for feed list (GROUP BY feed_id WHERE is_read = 0).
CREATE INDEX IF NOT EXISTS idx_articles_feed_unread
  ON articles(feed_id)
  WHERE is_read = 0;
