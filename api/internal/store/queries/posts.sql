-- name: ListPublishedPosts :many
SELECT p.id, p.slug, p.title, p.summary, p.word_count,
       p.published_at::timestamptz AS published_at, p.updated_at, p.cover_media_id,
       s.slug AS series_slug, s.title AS series_title
FROM posts p
LEFT JOIN series s ON s.id = p.series_id
WHERE p.status = 'published' AND p.published_at <= now()
  AND (sqlc.narg(tag_slug)::text IS NULL OR EXISTS (
        SELECT 1 FROM post_tags pt JOIN tags t ON t.id = pt.tag_id
        WHERE pt.post_id = p.id AND t.slug = sqlc.narg(tag_slug)::text))
  AND (sqlc.narg(cursor_ts)::timestamptz IS NULL
       OR (p.published_at, p.id) < (sqlc.narg(cursor_ts)::timestamptz, sqlc.narg(cursor_id)::bigint))
ORDER BY p.published_at DESC, p.id DESC
LIMIT @lim;

-- name: GetPublishedPostBySlug :one
SELECT p.id, p.slug, p.title, p.summary, p.body_html, p.toc, p.has_math, p.word_count,
       p.published_at::timestamptz AS published_at, p.updated_at, p.cover_media_id,
       p.series_id, s.slug AS series_slug, s.title AS series_title
FROM posts p
LEFT JOIN series s ON s.id = p.series_id
WHERE p.slug = @slug AND p.status = 'published' AND p.published_at <= now();

-- name: ResolvePublishedSlugAlias :one
SELECT p.slug
FROM post_slug_history h
JOIN posts p ON p.id = h.post_id
WHERE h.old_slug = @old_slug AND p.status = 'published' AND p.published_at <= now();

-- name: GetNewerPublishedPost :one
SELECT slug, title FROM posts
WHERE status = 'published' AND published_at <= now()
  AND (published_at, id) > (@published_at::timestamptz, @id::bigint)
ORDER BY published_at ASC, id ASC
LIMIT 1;

-- name: GetOlderPublishedPost :one
SELECT slug, title FROM posts
WHERE status = 'published' AND published_at <= now()
  AND (published_at, id) < (@published_at::timestamptz, @id::bigint)
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: SearchPublishedPosts :many
SELECT p.slug, p.title, p.summary, p.published_at::timestamptz AS published_at,
       greatest(m.pos - 40, 1)::int AS snippet_start,
       substr(p.plain_text, greatest(m.pos - 40, 1), 120)::text AS snippet,
       char_length(p.plain_text)::int AS plain_length
FROM posts p
CROSS JOIN LATERAL (SELECT strpos(lower(p.plain_text), lower(@q::text)) AS pos) m
WHERE p.status = 'published' AND p.published_at <= now()
  AND (p.title ILIKE @pattern::text OR p.summary ILIKE @pattern::text OR p.plain_text ILIKE @pattern::text)
ORDER BY greatest(3 * word_similarity(@q::text, p.title),
                  2 * word_similarity(@q::text, p.summary),
                  word_similarity(@q::text, p.plain_text)) DESC,
         p.published_at DESC, p.id DESC
LIMIT @lim;

-- name: ListAdminPosts :many
SELECT p.id, p.slug, p.title,
       (CASE WHEN p.status = 'draft' THEN 'draft'
             WHEN p.published_at > now() THEN 'scheduled'
             ELSE 'published' END)::text AS state,
       p.published_at, p.renderer_version, p.series_id, p.series_position, s.title AS series_title,
       p.version, p.created_at, p.updated_at
FROM posts p
LEFT JOIN series s ON s.id = p.series_id
WHERE (sqlc.narg(state)::text IS NULL
       OR (sqlc.narg(state)::text = 'draft' AND p.status = 'draft')
       OR (sqlc.narg(state)::text = 'scheduled' AND p.status = 'published' AND p.published_at > now())
       OR (sqlc.narg(state)::text = 'published' AND p.status = 'published' AND p.published_at <= now()))
  AND (sqlc.narg(title_pattern)::text IS NULL OR p.title ILIKE sqlc.narg(title_pattern)::text)
  AND (sqlc.narg(tag_id)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id AND pt.tag_id = sqlc.narg(tag_id)::bigint))
  AND (sqlc.narg(renderer_version_not)::text IS NULL OR p.renderer_version <> sqlc.narg(renderer_version_not)::text)
  AND (sqlc.narg(cursor_ts)::timestamptz IS NULL
       OR (p.updated_at, p.id) < (sqlc.narg(cursor_ts)::timestamptz, sqlc.narg(cursor_id)::bigint))
ORDER BY p.updated_at DESC, p.id DESC
LIMIT @lim;

-- name: GetAdminPostSummary :one
SELECT p.id, p.slug, p.title,
       (CASE WHEN p.status = 'draft' THEN 'draft'
             WHEN p.published_at > now() THEN 'scheduled'
             ELSE 'published' END)::text AS state,
       p.published_at, p.renderer_version, p.series_id, p.series_position, s.title AS series_title,
       p.version, p.created_at, p.updated_at
FROM posts p
LEFT JOIN series s ON s.id = p.series_id
WHERE p.id = @id;

-- name: GetAdminPost :one
SELECT p.id, p.slug, p.title, p.summary, p.body_md,
       (CASE WHEN p.status = 'draft' THEN 'draft'
             WHEN p.published_at > now() THEN 'scheduled'
             ELSE 'published' END)::text AS state,
       p.published_at, p.renderer_version, p.cover_media_id,
       p.series_id, p.series_position, s.title AS series_title,
       p.version, p.created_at, p.updated_at
FROM posts p
LEFT JOIN series s ON s.id = p.series_id
WHERE p.id = @id;

-- name: LockPost :one
SELECT id, slug, series_id, series_position, status, published_at, (body_md <> '')::bool AS has_body,
       version, updated_at
FROM posts
WHERE id = @id
FOR UPDATE;

-- name: InsertPost :one
INSERT INTO posts (slug, title, summary, body_md, body_html, toc, plain_text, word_count, has_math,
                   renderer_version, cover_media_id, series_id, series_position)
VALUES (@slug, @title, @summary, @body_md, @body_html, @toc, @plain_text, @word_count, @has_math,
        @renderer_version, @cover_media_id, @series_id, @series_position)
RETURNING id;

-- name: UpdatePostContent :exec
UPDATE posts
SET slug = @slug, title = @title, summary = @summary, body_md = @body_md, body_html = @body_html,
    toc = @toc, plain_text = @plain_text, word_count = @word_count, has_math = @has_math,
    renderer_version = @renderer_version, cover_media_id = @cover_media_id,
    series_id = @series_id, series_position = @series_position,
    version = version + 1, updated_at = now()
WHERE id = @id;

-- name: PublishPost :exec
UPDATE posts
SET status = 'published', published_at = @published_at::timestamptz, version = version + 1, updated_at = now()
WHERE id = @id;

-- name: UnpublishPost :exec
UPDATE posts SET status = 'draft', version = version + 1, updated_at = now() WHERE id = @id;

-- name: DeletePost :exec
DELETE FROM posts WHERE id = @id;

-- name: UpsertSlugHistory :exec
INSERT INTO post_slug_history (old_slug, post_id) VALUES (@old_slug, @post_id)
ON CONFLICT (old_slug) DO UPDATE SET post_id = EXCLUDED.post_id, created_at = now();

-- name: DeleteSlugHistory :exec
DELETE FROM post_slug_history WHERE old_slug = @old_slug;

-- name: DeletePostTags :exec
DELETE FROM post_tags WHERE post_id = @post_id;

-- name: InsertPostTags :exec
INSERT INTO post_tags (post_id, tag_id)
SELECT @post_id, unnest(@tag_ids::bigint[])
ON CONFLICT DO NOTHING;

-- name: DeletePostMedia :exec
DELETE FROM post_media WHERE post_id = @post_id;

-- name: InsertPostMedia :exec
INSERT INTO post_media (post_id, media_id)
SELECT @post_id, unnest(@media_ids::bigint[])
ON CONFLICT DO NOTHING;
