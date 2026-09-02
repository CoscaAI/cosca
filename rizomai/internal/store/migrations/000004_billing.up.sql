-- ============================================================================
-- RIZOMAI — migration 000004: BILLING (ADR-010)
--   * plans: catálogo de planos (free/growth/escala/agencia)
--   * team_plans: plano atual de cada team + período de billing
--   * usage_account_days: metering account-day (1 unidade por conta/dia,
--     idempotente — unique team+account+day)
--   * invoices: faturas por período (Stripe ou manual)
--   * metering_events: snapshot diário p/ cálculo da fatura
-- ============================================================================

-- --- plans ------------------------------------------------------------------
CREATE TABLE plans (
    codename                   TEXT PRIMARY KEY,  -- free | growth | escala | agencia
    name                       TEXT NOT NULL,
    contas_incluidas           INTEGER NOT NULL DEFAULT 3,
    preco_por_conta_extra_cents BIGINT NOT NULL DEFAULT 0,  -- ex.: 400 = R$4/conta excedente
    preco_fixo_cents           BIGINT NOT NULL DEFAULT 0,   -- ex.: agencia = R$49/mês
    stripe_price_id            TEXT,                        -- preenchido via env no seed
    features                   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- team_plans --------------------------------------------------------------
CREATE TABLE team_plans (
    team_id              TEXT PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
    plan_id              TEXT NOT NULL REFERENCES plans(codename),
    status               TEXT NOT NULL DEFAULT 'active',  -- active|trialing|past_due|canceled
    billing_anchor_day   INTEGER NOT NULL DEFAULT 1,
    current_period_start TIMESTAMPTZ NOT NULL DEFAULT now(),
    current_period_end   TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- usage_account_days (metering — ADR-010 §1.2) ----------------------------
CREATE TABLE usage_account_days (
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, account_id, day)
);
CREATE INDEX idx_usage_team_day ON usage_account_days (team_id, day);

-- --- invoices ----------------------------------------------------------------
CREATE TABLE invoices (
    id                    TEXT PRIMARY KEY,
    team_id               TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    period_start          DATE NOT NULL,
    period_end            DATE NOT NULL,
    valor_cents           BIGINT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'open',   -- open|paid|void
    stripe_invoice_id     TEXT,
    stripe_payment_intent TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_invoices_team ON invoices (team_id, period_end DESC);

-- --- metering_events (snapshot diário p/ fatura) ------------------------------
CREATE TABLE metering_events (
    id            TEXT PRIMARY KEY,
    team_id       TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    day           DATE NOT NULL,
    contas_ativas INTEGER NOT NULL DEFAULT 0,
    valor_cents   BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, day)
);

-- --- Seed do catálogo (ADR-010 §1.1 + produto §9.1) --------------------------
-- Preço POR CONTA EXCEDENTE sobre as incluídas (modelo simplificado de faixa
-- única por plano; a gradação total 3+7×$4+10×$2 é TODO no cálculo de fatura).
INSERT INTO plans (codename, name, contas_incluidas, preco_por_conta_extra_cents, preco_fixo_cents, features) VALUES
  ('free',    'Free',    3,  0,    0,    '{"postsIlimitados": true, "whiteLabel": false}'),
  ('growth',  'Growth',  3,  400,  0,    '{"postsIlimitados": true, "whiteLabel": false}'),
  ('escala',  'Escala',  3,  200,  0,    '{"postsIlimitados": true, "whiteLabel": false}'),
  ('agencia', 'Agência', 10, 0,    4900, '{"postsIlimitados": true, "whiteLabel": true}')
ON CONFLICT (codename) DO NOTHING;
