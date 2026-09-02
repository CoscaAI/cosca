-- ============================================================================
-- RIZOMAI — migration 000005: ANALYTICS + INBOX (Fase 6)
--   * post_analytics: métricas por (post, plataforma, dia) — snapshot diário
--   * inbox_messages: caixa unificada de DMs/comentários/menções
-- ============================================================================

CREATE TABLE post_analytics (
    post_id      TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    platform     platform NOT NULL,
    external_id  TEXT,
    views        BIGINT NOT NULL DEFAULT 0,
    likes        BIGINT NOT NULL DEFAULT 0,
    comments     BIGINT NOT NULL DEFAULT 0,
    shares       BIGINT NOT NULL DEFAULT 0,
    captured_at  DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, platform, captured_at)
);
CREATE INDEX idx_analytics_post ON post_analytics (post_id, captured_at DESC);

CREATE TABLE inbox_messages (
    id           TEXT PRIMARY KEY,
    profile_id   TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    platform     platform NOT NULL,
    external_id  TEXT,
    sender       TEXT NOT NULL,
    text         TEXT NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'dm',   -- dm | comment | mention
    is_read      BOOLEAN NOT NULL DEFAULT FALSE,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_inbox_profile ON inbox_messages (profile_id, is_read, received_at DESC);
