-- ============================================================================
-- RIZOMAI — migration 000001: schema inicial (ADR-002/007)
-- IDs textuais com prefixo (profile_, post_, target_...) gerados em Go
-- (ADR-005 §1.4). Timestamps timestamptz. JSONB para dados flexíveis.
-- ============================================================================

-- --- Enums ----------------------------------------------------------------
-- target_status NÃO inclui 'partial': partial é status AGREGADO de post
-- (ADR-007 §1), derivado dos targets.
CREATE TYPE platform AS ENUM ('x', 'linkedin', 'telegram');
CREATE TYPE target_status AS ENUM ('pending', 'scheduled', 'publishing', 'published', 'failed', 'skipped');
CREATE TYPE post_status AS ENUM ('scheduled', 'publishing', 'published', 'partial', 'failed', 'cancelled');
-- publish_outcome: base do ADR-007 (success|failed|timeout) + rate_limited/skipped
-- (429 é retryable — ADR-003 — e merece outcome próprio no log de tentativas).
CREATE TYPE publish_outcome AS ENUM ('success', 'failed', 'timeout', 'rate_limited', 'skipped');
CREATE TYPE token_status AS ENUM ('ok', 'expired', 'revoked', 'needs_attention');

-- --- teams -----------------------------------------------------------------
CREATE TABLE teams (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- profiles --------------------------------------------------------------
CREATE TABLE profiles (
    id         TEXT PRIMARY KEY,
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_profiles_team ON profiles (team_id);

-- --- social_accounts (ADR-006) ---------------------------------------------
-- encrypted_token: BYTEA — AES-256-GCM em repouso entra na Fase 3 (OAuth broker).
CREATE TABLE social_accounts (
    id               TEXT PRIMARY KEY,
    profile_id       TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    platform         platform NOT NULL,
    display_name     TEXT,
    platform_user_id TEXT,
    token_status     token_status NOT NULL DEFAULT 'ok',
    encrypted_token  BYTEA,
    settings         JSONB NOT NULL DEFAULT '{}'::jsonb,
    connected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_accounts_profile ON social_accounts (profile_id);
CREATE INDEX idx_accounts_platform ON social_accounts (platform);

-- --- posts (ADR-007: conteúdo + status agregado materializado) --------------
CREATE TABLE posts (
    id           TEXT PRIMARY KEY,
    profile_id   TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    content      TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
    media_urls   JSONB NOT NULL DEFAULT '[]'::jsonb,
    scheduled_for TIMESTAMPTZ,
    timezone     TEXT NOT NULL DEFAULT 'UTC',
    status       post_status NOT NULL DEFAULT 'scheduled',
    content_hash TEXT,   -- idempotência content-hash (ADR-005 §1.3)
    created_by   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_posts_profile_status  ON posts (profile_id, status);
CREATE INDEX idx_posts_profile_created ON posts (profile_id, created_at DESC);
CREATE INDEX idx_posts_content_hash    ON posts (content_hash) WHERE content_hash IS NOT NULL;

-- --- post_targets (ADR-007: 1 linha por (post, account, platform)) ----------
CREATE TABLE post_targets (
    id                     TEXT PRIMARY KEY,
    post_id                TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    account_id             TEXT NOT NULL REFERENCES social_accounts(id),
    platform               platform NOT NULL,
    status                 target_status NOT NULL DEFAULT 'pending',
    platform_specific_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_url          TEXT,
    external_post_id       TEXT,
    last_error             JSONB,          -- { "code": "...", "error": "..." }
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (post_id, account_id, platform)
);
CREATE INDEX idx_targets_post    ON post_targets (post_id);
CREATE INDEX idx_targets_status  ON post_targets (status);
CREATE INDEX idx_targets_account ON post_targets (account_id);

-- --- publish_attempts (ADR-007: log append-only) ----------------------------
CREATE TABLE publish_attempts (
    id          TEXT PRIMARY KEY,
    target_id   TEXT NOT NULL REFERENCES post_targets(id) ON DELETE CASCADE,
    attempt     INTEGER NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    outcome     publish_outcome NOT NULL,
    error       JSONB,
    http_status INTEGER,
    request_id  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (target_id, attempt)
);
CREATE INDEX idx_attempts_target ON publish_attempts (target_id, attempt);

-- --- media (ADR-008) ---------------------------------------------------------
CREATE TABLE media (
    id             TEXT PRIMARY KEY,
    profile_id     TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    filename       TEXT NOT NULL,
    content_type   TEXT NOT NULL,
    size_bytes     BIGINT NOT NULL,
    storage_path   TEXT NOT NULL,
    public_url     TEXT NOT NULL,
    retention_days INTEGER,
    expires_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_media_profile ON media (profile_id);

-- --- webhooks (ADR-009) ------------------------------------------------------
CREATE TABLE webhooks (
    id             TEXT PRIMARY KEY,
    profile_id     TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    url            TEXT NOT NULL,
    secret_hash    TEXT NOT NULL,  -- HMAC secret: armazenado HASHED (ADR-009 §1.7)
    events         JSONB NOT NULL DEFAULT '["post.published","post.failed"]'::jsonb,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    custom_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhooks_profile ON webhooks (profile_id);

-- --- api_keys (ADR-006: cliente autentica com API key sk_...) ---------------
CREATE TABLE api_keys (
    id           TEXT PRIMARY KEY,
    team_id      TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,  -- SHA-256(pepper || key); nunca plaintext
    key_prefix   TEXT NOT NULL,         -- ex.: sk_live_4f2a — identificação visual
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_keys_team ON api_keys (team_id);

-- --- idempotency_keys (ADR-005 §1.3: mesma Idempotency-Key → mesma resposta) -
CREATE TABLE idempotency_keys (
    key          TEXT NOT NULL,
    team_id      TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    request_hash TEXT NOT NULL,
    status_code  INTEGER NOT NULL,
    response     JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key, team_id)
);
