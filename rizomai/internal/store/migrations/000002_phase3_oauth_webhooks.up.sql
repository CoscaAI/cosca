-- ============================================================================
-- RIZOMAI — migration 000002: Fase 3 — OAuth broker (ADR-006) e webhooks (ADR-009)
--   * oauth_states: state + PKCE verifier para o callback server-side
--   * social_accounts: credenciais criptografadas AES-256-GCM + escopo + expiração
--   * webhooks: secret criptografado (necessário para ASSINAR payloads)
--   * webhook_deliveries: log de cada tentativa de entrega
-- ============================================================================

-- --- oauth_states -----------------------------------------------------------
CREATE TABLE oauth_states (
    id            TEXT PRIMARY KEY,
    team_id       TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    platform      platform NOT NULL,
    state         TEXT NOT NULL UNIQUE,
    code_verifier TEXT,            -- PKCE (X) — nunca confiar em sessão (insumo §6.1)
    redirect_uri  TEXT,
    profile_id    TEXT REFERENCES profiles(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_oauth_states_team ON oauth_states (team_id);

-- --- social_accounts: credenciais criptografadas (ADR-006 §1.2) -------------
-- encrypted_token (access token) já existe desde a 000001 (BYTEA).
ALTER TABLE social_accounts
    ADD COLUMN refresh_token_encrypted BYTEA,
    ADD COLUMN expires_at TIMESTAMPTZ,
    ADD COLUMN external_identifier TEXT,   -- chat_id (telegram), author URN (linkedin), user id (x)
    ADD COLUMN token_scope TEXT;

-- --- webhooks: secret criptografado para assinatura HMAC (ADR-009 §1.2) ------
ALTER TABLE webhooks
    ADD COLUMN secret_encrypted BYTEA;

-- --- webhook_deliveries: log append-only de entregas (ADR-009 §1.5) ----------
CREATE TABLE webhook_deliveries (
    id          TEXT PRIMARY KEY,
    webhook_id  TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_id    TEXT NOT NULL,             -- MESMO id em todos os retries (dedup do consumidor)
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL,
    status      TEXT NOT NULL,             -- success | failed
    http_status INTEGER,
    error       TEXT,
    attempts    INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_event   ON webhook_deliveries (event_id);
