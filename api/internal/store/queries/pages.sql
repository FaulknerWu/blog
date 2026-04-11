-- name: GetPublishedPageBySlug :one
SELECT slug, title, description, body_html, toc, has_math, updated_at
FROM pages
WHERE slug = @slug AND status = 'published';

-- name: ResolvePublishedPageSlugAlias :one
SELECT p.slug
FROM page_slug_history h
JOIN pages p ON p.id = h.page_id
WHERE h.old_slug = @old_slug AND p.status = 'published';

-- name: ListAdminPages :many
SELECT id, slug, title, status, renderer_version, version, created_at, updated_at
FROM pages
WHERE sqlc.narg(renderer_version_not)::text IS NULL OR renderer_version <> sqlc.narg(renderer_version_not)::text
ORDER BY updated_at DESC, id DESC;

-- name: GetAdminPage :one
SELECT id, slug, title, status, renderer_version, version, created_at, updated_at, description, body_md
FROM pages
WHERE id = @id;

-- name: LockPage :one
SELECT id, slug, status, (body_md <> '')::bool AS has_body, version, updated_at
FROM pages
WHERE id = @id
FOR UPDATE;

-- name: InsertPage :one
INSERT INTO pages (slug, title, description, body_md, body_html, toc, plain_text, has_math, renderer_version)
VALUES (@slug, @title, @description, @body_md, @body_html, @toc, @plain_text, @has_math, @renderer_version)
RETURNING id;

-- name: UpdatePageContent :exec
UPDATE pages
SET slug = @slug, title = @title, description = @description, body_md = @body_md, body_html = @body_html,
    toc = @toc, plain_text = @plain_text, has_math = @has_math, renderer_version = @renderer_version,
    version = version + 1, updated_at = now()
WHERE id = @id;

-- name: SetPageStatus :exec
UPDATE pages SET status = @status, version = version + 1, updated_at = now() WHERE id = @id;

-- name: DeletePage :exec
DELETE FROM pages WHERE id = @id;

-- name: UpsertPageSlugHistory :exec
INSERT INTO page_slug_history (old_slug, page_id) VALUES (@old_slug, @page_id)
ON CONFLICT (old_slug) DO UPDATE SET page_id = EXCLUDED.page_id, created_at = now();

-- name: DeletePageSlugHistory :exec
DELETE FROM page_slug_history WHERE old_slug = @old_slug;

-- name: DeletePageMedia :exec
DELETE FROM page_media WHERE page_id = @page_id;

-- name: InsertPageMedia :exec
INSERT INTO page_media (page_id, media_id)
SELECT @page_id, unnest(@media_ids::bigint[])
ON CONFLICT DO NOTHING;
