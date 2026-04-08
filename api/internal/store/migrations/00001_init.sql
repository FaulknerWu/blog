-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY,
    username      text        NOT NULL,
    display_name  text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_username_check CHECK (username ~ '^[a-z0-9_]{3,32}$')
);

CREATE TABLE sessions (
    id           bigint GENERATED ALWAYS AS IDENTITY,
    token_hash   bytea       NOT NULL,
    user_id      bigint      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    user_agent   text        NOT NULL DEFAULT '',
    ip           inet,
    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE series (
    id          bigint GENERATED ALWAYS AS IDENTITY,
    slug        text        NOT NULL,
    title       text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    version     integer     NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT series_pkey PRIMARY KEY (id),
    CONSTRAINT series_slug_key UNIQUE (slug),
    CONSTRAINT series_slug_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 100),
    CONSTRAINT series_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT series_description_check CHECK (char_length(description) <= 2000)
);

CREATE TABLE media (
    id                bigint GENERATED ALWAYS AS IDENTITY,
    sha256            text        NOT NULL,
    width             integer     NOT NULL,
    height            integer     NOT NULL,
    byte_size         bigint      NOT NULL,
    variants          jsonb       NOT NULL,
    original_filename text        NOT NULL DEFAULT '',
    alt               text        NOT NULL DEFAULT '',
    version           integer     NOT NULL DEFAULT 1,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_pkey PRIMARY KEY (id),
    CONSTRAINT media_sha256_key UNIQUE (sha256),
    CONSTRAINT media_sha256_check CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT media_width_check CHECK (width > 0),
    CONSTRAINT media_height_check CHECK (height > 0),
    CONSTRAINT media_byte_size_check CHECK (byte_size > 0),
    CONSTRAINT media_variants_check CHECK (jsonb_typeof(variants) = 'array'),
    CONSTRAINT media_alt_check CHECK (char_length(alt) <= 500)
);
CREATE INDEX media_created_idx ON media (created_at DESC, id DESC);

CREATE TABLE posts (
    id               bigint GENERATED ALWAYS AS IDENTITY,
    slug             text        NOT NULL,
    title            text        NOT NULL,
    summary          text        NOT NULL DEFAULT '',
    body_md          text        NOT NULL DEFAULT '',
    body_html        text        NOT NULL DEFAULT '',
    toc              jsonb       NOT NULL DEFAULT '[]',
    plain_text       text        NOT NULL DEFAULT '',
    word_count       integer     NOT NULL DEFAULT 0,
    has_math         boolean     NOT NULL DEFAULT false,
    renderer_version text        NOT NULL DEFAULT '',
    cover_media_id   bigint,
    series_id        bigint,
    series_position  integer,
    status           text        NOT NULL DEFAULT 'draft',
    published_at     timestamptz,
    version          integer     NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT posts_pkey PRIMARY KEY (id),
    CONSTRAINT posts_slug_key UNIQUE (slug),
    CONSTRAINT posts_slug_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 100),
    CONSTRAINT posts_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT posts_summary_check CHECK (char_length(summary) <= 500),
    CONSTRAINT posts_toc_check CHECK (jsonb_typeof(toc) = 'array'),
    CONSTRAINT posts_word_count_check CHECK (word_count >= 0),
    CONSTRAINT posts_status_check CHECK (status IN ('draft', 'published')),
    CONSTRAINT posts_published_at_required CHECK (status = 'draft' OR published_at IS NOT NULL),
    CONSTRAINT posts_series_pair CHECK ((series_id IS NULL) = (series_position IS NULL)),
    CONSTRAINT posts_series_position_key UNIQUE (series_id, series_position) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT posts_cover_media_id_fkey FOREIGN KEY (cover_media_id) REFERENCES media (id) ON DELETE RESTRICT,
    CONSTRAINT posts_series_id_fkey FOREIGN KEY (series_id) REFERENCES series (id) ON DELETE RESTRICT
);
CREATE INDEX posts_published_idx ON posts (published_at DESC, id DESC) WHERE status = 'published';
CREATE INDEX posts_updated_idx ON posts (updated_at DESC, id DESC);
CREATE INDEX posts_title_trgm_idx ON posts USING gin (title gin_trgm_ops);
CREATE INDEX posts_summary_trgm_idx ON posts USING gin (summary gin_trgm_ops);
CREATE INDEX posts_plain_text_trgm_idx ON posts USING gin (plain_text gin_trgm_ops);
CREATE INDEX posts_cover_media_id_idx ON posts (cover_media_id) WHERE cover_media_id IS NOT NULL;

CREATE TABLE post_slug_history (
    old_slug   text        NOT NULL,
    post_id    bigint      NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT post_slug_history_pkey PRIMARY KEY (old_slug),
    CONSTRAINT post_slug_history_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE
);
CREATE INDEX post_slug_history_post_id_idx ON post_slug_history (post_id);

CREATE TABLE tags (
    id         bigint GENERATED ALWAYS AS IDENTITY,
    slug       text        NOT NULL,
    name       text        NOT NULL,
    version    integer     NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tags_pkey PRIMARY KEY (id),
    CONSTRAINT tags_slug_key UNIQUE (slug),
    CONSTRAINT tags_slug_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 50),
    CONSTRAINT tags_name_check CHECK (char_length(name) BETWEEN 1 AND 50)
);
CREATE UNIQUE INDEX tags_name_lower_key ON tags (lower(name));

CREATE TABLE post_tags (
    post_id bigint NOT NULL,
    tag_id  bigint NOT NULL,
    CONSTRAINT post_tags_pkey PRIMARY KEY (post_id, tag_id),
    CONSTRAINT post_tags_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE,
    CONSTRAINT post_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
);
CREATE INDEX post_tags_tag_id_idx ON post_tags (tag_id, post_id);

CREATE TABLE post_media (
    post_id  bigint NOT NULL,
    media_id bigint NOT NULL,
    CONSTRAINT post_media_pkey PRIMARY KEY (post_id, media_id),
    CONSTRAINT post_media_post_id_fkey FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE,
    CONSTRAINT post_media_media_id_fkey FOREIGN KEY (media_id) REFERENCES media (id) ON DELETE RESTRICT
);
CREATE INDEX post_media_media_id_idx ON post_media (media_id);

CREATE TABLE pages (
    id               bigint GENERATED ALWAYS AS IDENTITY,
    slug             text        NOT NULL,
    title            text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    body_md          text        NOT NULL DEFAULT '',
    body_html        text        NOT NULL DEFAULT '',
    toc              jsonb       NOT NULL DEFAULT '[]',
    plain_text       text        NOT NULL DEFAULT '',
    has_math         boolean     NOT NULL DEFAULT false,
    renderer_version text        NOT NULL DEFAULT '',
    status           text        NOT NULL DEFAULT 'draft',
    version          integer     NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pages_pkey PRIMARY KEY (id),
    CONSTRAINT pages_slug_key UNIQUE (slug),
    CONSTRAINT pages_slug_check CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 100),
    CONSTRAINT pages_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT pages_description_check CHECK (char_length(description) <= 500),
    CONSTRAINT pages_toc_check CHECK (jsonb_typeof(toc) = 'array'),
    CONSTRAINT pages_status_check CHECK (status IN ('draft', 'published'))
);

CREATE TABLE page_slug_history (
    old_slug   text        NOT NULL,
    page_id    bigint      NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT page_slug_history_pkey PRIMARY KEY (old_slug),
    CONSTRAINT page_slug_history_page_id_fkey FOREIGN KEY (page_id) REFERENCES pages (id) ON DELETE CASCADE
);
CREATE INDEX page_slug_history_page_id_idx ON page_slug_history (page_id);

CREATE TABLE page_media (
    page_id  bigint NOT NULL,
    media_id bigint NOT NULL,
    CONSTRAINT page_media_pkey PRIMARY KEY (page_id, media_id),
    CONSTRAINT page_media_page_id_fkey FOREIGN KEY (page_id) REFERENCES pages (id) ON DELETE CASCADE,
    CONSTRAINT page_media_media_id_fkey FOREIGN KEY (media_id) REFERENCES media (id) ON DELETE RESTRICT
);
CREATE INDEX page_media_media_id_idx ON page_media (media_id);

CREATE VIEW media_references AS
SELECT pm.media_id, 'post'::text AS owner_type, p.id AS owner_id, p.title
FROM post_media pm
JOIN posts p ON p.id = pm.post_id
UNION
SELECT p.cover_media_id, 'post'::text, p.id, p.title
FROM posts p
WHERE p.cover_media_id IS NOT NULL
UNION
SELECT pm.media_id, 'page'::text, pg.id, pg.title
FROM page_media pm
JOIN pages pg ON pg.id = pm.page_id;

CREATE TABLE site_settings (
    id              smallint    NOT NULL DEFAULT 1,
    title           text        NOT NULL DEFAULT 'Blog',
    subtitle        text        NOT NULL DEFAULT '',
    description     text        NOT NULL DEFAULT '',
    author          text        NOT NULL DEFAULT 'Author',
    language        text        NOT NULL DEFAULT 'zh-CN',
    timezone        text        NOT NULL DEFAULT 'Asia/Shanghai',
    footer_text     text        NOT NULL DEFAULT '',
    navigation      jsonb       NOT NULL DEFAULT '[]',
    social_links    jsonb       NOT NULL DEFAULT '[]',
    noindex         boolean     NOT NULL DEFAULT false,
    posts_per_page  integer     NOT NULL DEFAULT 20,
    feed_item_count integer     NOT NULL DEFAULT 20,
    version         integer     NOT NULL DEFAULT 1,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT site_settings_pkey PRIMARY KEY (id),
    CONSTRAINT site_settings_singleton_check CHECK (id = 1),
    CONSTRAINT site_settings_title_check CHECK (char_length(title) BETWEEN 1 AND 100),
    CONSTRAINT site_settings_subtitle_check CHECK (char_length(subtitle) <= 200),
    CONSTRAINT site_settings_description_check CHECK (char_length(description) <= 500),
    CONSTRAINT site_settings_author_check CHECK (char_length(author) BETWEEN 1 AND 100),
    CONSTRAINT site_settings_language_check CHECK (language ~ '^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$' AND char_length(language) <= 35),
    CONSTRAINT site_settings_timezone_check CHECK (char_length(timezone) BETWEEN 1 AND 64),
    CONSTRAINT site_settings_footer_text_check CHECK (char_length(footer_text) <= 500),
    CONSTRAINT site_settings_navigation_check CHECK (jsonb_typeof(navigation) = 'array' AND jsonb_array_length(navigation) <= 10),
    CONSTRAINT site_settings_social_links_check CHECK (jsonb_typeof(social_links) = 'array' AND jsonb_array_length(social_links) <= 10),
    CONSTRAINT site_settings_posts_per_page_check CHECK (posts_per_page BETWEEN 1 AND 50),
    CONSTRAINT site_settings_feed_item_count_check CHECK (feed_item_count BETWEEN 1 AND 100)
);
INSERT INTO site_settings DEFAULT VALUES;

-- +goose Down
DROP TABLE site_settings;
DROP VIEW media_references;
DROP TABLE page_media;
DROP TABLE page_slug_history;
DROP TABLE pages;
DROP TABLE post_media;
DROP TABLE post_tags;
DROP TABLE tags;
DROP TABLE post_slug_history;
DROP TABLE posts;
DROP TABLE media;
DROP TABLE series;
DROP TABLE sessions;
DROP TABLE users;
