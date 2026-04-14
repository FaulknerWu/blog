-- name: ListFeedPosts :many
SELECT id, slug, title, summary, body_html, published_at::timestamptz AS published_at, updated_at, version
FROM posts
WHERE status = 'published' AND published_at <= now()
ORDER BY published_at DESC, id DESC
LIMIT @lim;

-- name: ListSitemapPosts :many
SELECT slug, updated_at
FROM posts
WHERE status = 'published' AND published_at <= now()
ORDER BY published_at DESC, id DESC;

-- name: ListSitemapTags :many
SELECT t.slug, max(greatest(p.updated_at, t.updated_at))::timestamptz AS last_modified
FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE p.status = 'published' AND p.published_at <= now()
GROUP BY t.id
ORDER BY t.slug;

-- name: ListSitemapSeries :many
SELECT s.slug, max(greatest(p.updated_at, s.updated_at))::timestamptz AS last_modified
FROM series s
JOIN posts p ON p.series_id = s.id
WHERE p.status = 'published' AND p.published_at <= now()
GROUP BY s.id
ORDER BY s.slug;


-- name: ListSitemapPages :many
SELECT slug, updated_at
FROM pages
WHERE status = 'published'
ORDER BY slug;
