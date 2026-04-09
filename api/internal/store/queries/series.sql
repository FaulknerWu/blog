-- name: ListAdminSeries :many
SELECT s.id, s.slug, s.title, s.description, s.version, s.created_at, s.updated_at,
       count(p.id)::int AS post_count,
       (count(p.id) FILTER (WHERE p.status = 'published' AND p.published_at <= now()))::int AS published_post_count
FROM series s
LEFT JOIN posts p ON p.series_id = s.id
GROUP BY s.id
ORDER BY s.created_at DESC, s.id DESC;

-- name: GetAdminSeries :one
SELECT s.id, s.slug, s.title, s.description, s.version, s.created_at, s.updated_at,
       count(p.id)::int AS post_count,
       (count(p.id) FILTER (WHERE p.status = 'published' AND p.published_at <= now()))::int AS published_post_count
FROM series s
LEFT JOIN posts p ON p.series_id = s.id
WHERE s.id = @id
GROUP BY s.id;

-- name: ListAdminSeriesPosts :many
SELECT p.id, p.slug, p.title, p.series_position::int AS position,
       (CASE WHEN p.status = 'draft' THEN 'draft'
             WHEN p.published_at > now() THEN 'scheduled'
             ELSE 'published' END)::text AS state
FROM posts p
WHERE p.series_id = @series_id::bigint
ORDER BY p.series_position, p.id;

-- name: LockSeries :one
SELECT id, slug, title, description, version, updated_at FROM series WHERE id = @id FOR UPDATE;

-- name: CreateSeries :one
INSERT INTO series (slug, title, description) VALUES (@slug, @title, @description) RETURNING id;

-- name: UpdateSeries :exec
UPDATE series
SET slug = @slug, title = @title, description = @description, version = version + 1, updated_at = now()
WHERE id = @id;

-- name: DetachSeriesPosts :exec
UPDATE posts SET series_id = NULL, series_position = NULL WHERE series_id = @series_id::bigint;

-- name: DeleteSeries :exec
DELETE FROM series WHERE id = @id;

-- name: NextSeriesPosition :one
SELECT (coalesce(max(series_position), 0) + 1)::int FROM posts WHERE series_id = @series_id::bigint;

-- name: SetSeriesPosition :exec
UPDATE posts SET series_position = @position::int WHERE id = @id AND series_id = @series_id::bigint;

-- name: ListPublicSeries :many
SELECT s.slug, s.title, s.description,
       count(*)::int AS post_count,
       max(p.published_at)::timestamptz AS last_published_at
FROM series s
JOIN posts p ON p.series_id = s.id
WHERE p.status = 'published' AND p.published_at <= now()
GROUP BY s.id
ORDER BY max(p.published_at) DESC, s.id DESC;

-- name: GetSeriesBySlug :one
SELECT id, slug, title, description FROM series WHERE slug = @slug;

-- name: ListPublishedSeriesPosts :many
SELECT p.id, p.slug, p.title, p.published_at::timestamptz AS published_at
FROM posts p
WHERE p.series_id = @series_id::bigint AND p.status = 'published' AND p.published_at <= now()
ORDER BY p.series_position, p.id;
