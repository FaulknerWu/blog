-- name: InsertMedia :one
INSERT INTO media (sha256, width, height, byte_size, variants, original_filename, alt)
VALUES (@sha256, @width, @height, @byte_size, @variants, @original_filename, @alt)
ON CONFLICT (sha256) DO NOTHING
RETURNING id;

-- name: GetMediaIDBySHA256 :one
SELECT id FROM media WHERE sha256 = @sha256;

-- name: GetMediaVariantsBySHA256 :one
SELECT variants, updated_at FROM media WHERE sha256 = @sha256;

-- name: GetMedia :one
SELECT m.*,
       (SELECT count(*) FROM media_references r WHERE r.media_id = m.id)::int AS reference_count
FROM media m
WHERE m.id = @id;

-- name: ListMedia :many
SELECT m.*,
       (SELECT count(*) FROM media_references r WHERE r.media_id = m.id)::int AS reference_count
FROM media m
WHERE (NOT @unused::bool OR NOT EXISTS (SELECT 1 FROM media_references r WHERE r.media_id = m.id))
  AND (sqlc.narg(cursor_ts)::timestamptz IS NULL
       OR (m.created_at, m.id) < (sqlc.narg(cursor_ts)::timestamptz, sqlc.narg(cursor_id)::bigint))
ORDER BY m.created_at DESC, m.id DESC
LIMIT @lim;

-- name: ListMediaBySHA256 :many
SELECT m.*,
       (SELECT count(*) FROM media_references r WHERE r.media_id = m.id)::int AS reference_count
FROM media m
WHERE m.sha256 = ANY(@sha256s::text[])
ORDER BY m.created_at DESC, m.id DESC;

-- name: ListMediaReferences :many
SELECT owner_type, owner_id, title
FROM media_references
WHERE media_id = @media_id::bigint
ORDER BY owner_type, owner_id;

-- name: ListMediaIDsBySHA256 :many
SELECT id, sha256 FROM media WHERE sha256 = ANY(@sha256s::text[]);

-- name: MediaExists :one
SELECT EXISTS (SELECT 1 FROM media WHERE id = @id);

-- name: ListImages :many
SELECT id, sha256, width, height, alt, variants FROM media WHERE id = ANY(@ids::bigint[]);

-- name: LockMedia :one
SELECT id, sha256, version, updated_at FROM media WHERE id = @id FOR UPDATE;

-- name: UpdateMediaAlt :exec
UPDATE media SET alt = @alt, version = version + 1, updated_at = now() WHERE id = @id;

-- name: DeleteMedia :exec
DELETE FROM media WHERE id = @id;
