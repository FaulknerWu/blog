-- name: ListAdminTags :many
SELECT t.id, t.slug, t.name, t.version, t.created_at, t.updated_at,
       count(p.id)::int AS post_count,
       (count(p.id) FILTER (WHERE p.status = 'published' AND p.published_at <= now()))::int AS published_post_count
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
LEFT JOIN posts p ON p.id = pt.post_id
GROUP BY t.id
ORDER BY lower(t.name), t.id;

-- name: GetAdminTag :one
SELECT t.id, t.slug, t.name, t.version, t.created_at, t.updated_at,
       count(p.id)::int AS post_count,
       (count(p.id) FILTER (WHERE p.status = 'published' AND p.published_at <= now()))::int AS published_post_count
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
LEFT JOIN posts p ON p.id = pt.post_id
WHERE t.id = @id
GROUP BY t.id;

-- name: LockTag :one
SELECT id, slug, name, version, updated_at FROM tags WHERE id = @id FOR UPDATE;

-- name: CreateTag :one
INSERT INTO tags (slug, name) VALUES (@slug, @name) RETURNING id;

-- name: UpdateTag :exec
UPDATE tags
SET slug = @slug, name = @name, version = version + 1, updated_at = now()
WHERE id = @id;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = @id;

-- name: ListTagIDs :many
SELECT id FROM tags WHERE id = ANY(@ids::bigint[]);

-- name: ListTagsForPosts :many
SELECT pt.post_id, t.id, t.slug, t.name
FROM post_tags pt
JOIN tags t ON t.id = pt.tag_id
WHERE pt.post_id = ANY(@post_ids::bigint[])
ORDER BY lower(t.name), t.id;

-- name: ListPublicTags :many
SELECT t.slug, t.name, count(*)::int AS post_count
FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE p.status = 'published' AND p.published_at <= now()
GROUP BY t.id
ORDER BY lower(t.name), t.id;

-- name: GetPublicTag :one
SELECT t.slug, t.name, count(*)::int AS post_count
FROM tags t
JOIN post_tags pt ON pt.tag_id = t.id
JOIN posts p ON p.id = pt.post_id
WHERE t.slug = @slug AND p.status = 'published' AND p.published_at <= now()
GROUP BY t.id;
