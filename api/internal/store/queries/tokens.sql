-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash, scopes, expires_at)
VALUES (@user_id, @name, @token_hash, @scopes, sqlc.narg(expires_at))
RETURNING *;

-- name: GetActiveAPITokenByHash :one
SELECT t.id, t.user_id, t.scopes, t.last_used_at, u.username, u.display_name
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = @id;

-- name: ListAPITokens :many
SELECT * FROM api_tokens
WHERE user_id = @user_id
ORDER BY created_at DESC, id DESC;

-- name: DeleteAPIToken :execrows
DELETE FROM api_tokens WHERE id = @id AND user_id = @user_id;

-- name: DeleteAPITokenByName :execrows
DELETE FROM api_tokens WHERE name = @name AND user_id = @user_id;

-- name: DeleteUserAPITokens :exec
DELETE FROM api_tokens WHERE user_id = @user_id;
