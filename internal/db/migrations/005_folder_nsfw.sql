-- NSFW / sensitive folder flag. When UIPrefs.nsfwMode is false (office mode),
-- sidebar hides the folder and its feeds; smart lists/search exclude their articles.
-- Background refresh still runs. Independent of feeds.is_nsfw (either side hides).

ALTER TABLE folders ADD COLUMN is_nsfw INTEGER NOT NULL DEFAULT 0;
