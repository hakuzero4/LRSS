-- NSFW / sensitive feed flag. When UIPrefs.nsfwMode is false (office mode),
-- sidebar and smart lists hide feeds with is_nsfw=1. Background refresh still runs.

ALTER TABLE feeds ADD COLUMN is_nsfw INTEGER NOT NULL DEFAULT 0;
