-- Persist full-article bilingual translation alongside original body.
-- content_html / content_text remain the feed original and are never overwritten by translate.

ALTER TABLE articles ADD COLUMN translation_raw TEXT;
ALTER TABLE articles ADD COLUMN translation_lang TEXT;
