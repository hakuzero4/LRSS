-- Per-feed refresh interval and user-locked display title.
-- refresh_interval_minutes: 0 = use global LibraryConfig default; otherwise 5–180.
-- title_user_set: when 1, refresh must not overwrite title from feed document.

ALTER TABLE feeds ADD COLUMN refresh_interval_minutes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN title_user_set INTEGER NOT NULL DEFAULT 0;
