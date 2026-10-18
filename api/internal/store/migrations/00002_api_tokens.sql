-- +goose Up
-- api_tokens 是给 CLI 与 agent 使用的长期凭据，以 Bearer 方式提交；与会话一样只保存 SHA-256。
CREATE TABLE api_tokens (
    id           bigint GENERATED ALWAYS AS IDENTITY,
    user_id      bigint      NOT NULL,
    name         text        NOT NULL,
    token_hash   bytea       NOT NULL,
    scopes       text[]      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at   timestamptz,
    CONSTRAINT api_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT api_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT api_tokens_user_name_key UNIQUE (user_id, name),
    CONSTRAINT api_tokens_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT api_tokens_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT api_tokens_scopes_check CHECK (
        cardinality(scopes) > 0
        AND scopes <@ ARRAY['content:read', 'content:write', 'content:publish', 'media:write', 'settings:write']
    ),
    CONSTRAINT api_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id);

-- +goose Down
DROP TABLE api_tokens;
