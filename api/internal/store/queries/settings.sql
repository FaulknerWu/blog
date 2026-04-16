-- name: GetSiteSettings :one
SELECT id, title, subtitle, description, author, language, timezone, footer_text, navigation, social_links,
       noindex, posts_per_page, feed_item_count, version, updated_at
FROM site_settings WHERE id = 1;

-- name: LockSiteSettings :one
SELECT id, title, subtitle, description, author, language, timezone, footer_text, navigation, social_links,
       noindex, posts_per_page, feed_item_count, version, updated_at
FROM site_settings WHERE id = 1 FOR UPDATE;

-- name: UpdateSiteSettings :exec
UPDATE site_settings
SET title = @title, subtitle = @subtitle, description = @description, author = @author, language = @language,
    timezone = @timezone, footer_text = @footer_text, navigation = @navigation, social_links = @social_links,
    noindex = @noindex, posts_per_page = @posts_per_page, feed_item_count = @feed_item_count,
    version = version + 1, updated_at = now()
WHERE id = 1;
