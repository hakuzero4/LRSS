-- 精选 article-tree folders. Deleting a folder SET NULLs article_keeps.folder_id (root).
-- Child folders CASCADE with their parent.

CREATE TABLE IF NOT EXISTS keep_folders (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  parent_id   TEXT REFERENCES keep_folders(id) ON DELETE CASCADE,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  hint        TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_keep_folders_parent ON keep_folders(parent_id);

ALTER TABLE article_keeps ADD COLUMN folder_id TEXT REFERENCES keep_folders(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_article_keeps_folder ON article_keeps(folder_id);
