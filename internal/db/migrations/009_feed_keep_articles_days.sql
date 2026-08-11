-- Per-feed article retention (days).
-- 0 = use global UIPrefs keepArticlesDays; otherwise clamped to [7, 365] at write time.

ALTER TABLE feeds ADD COLUMN keep_articles_days INTEGER NOT NULL DEFAULT 0;
