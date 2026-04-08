-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: CreateUser :one
INSERT INTO users (username, display_name, password_hash)
VALUES (@username, @display_name, @password_hash)
RETURNING *;

-- name: UpdateUserPassword :execrows
UPDATE users
SET password_hash = @password_hash, updated_at = now()
WHERE id = @id;

-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, expires_at, user_agent, ip)
VALUES (@token_hash, @user_id, @expires_at, @user_agent, @ip)
RETURNING *;

-- name: GetActiveSessionByTokenHash :one
SELECT s.id, s.user_id, s.created_at, s.last_seen_at, s.expires_at,
       u.username, u.display_name
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash AND s.expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), expires_at = @expires_at WHERE id = @id;

-- name: ListActiveSessions :many
SELECT * FROM sessions
WHERE user_id = @user_id AND expires_at > now()
ORDER BY last_seen_at DESC, id DESC;

-- name: DeleteUserSession :execrows
DELETE FROM sessions WHERE id = @id AND user_id = @user_id;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = @user_id AND id <> @keep_id;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();
