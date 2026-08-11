-- Mark articles whose body was replaced by full-page fetch (请求全文 / auto fetch).
-- Prevents re-detecting partial and toasting on every reopen.

ALTER TABLE articles ADD COLUMN full_content_fetched INTEGER NOT NULL DEFAULT 0;
