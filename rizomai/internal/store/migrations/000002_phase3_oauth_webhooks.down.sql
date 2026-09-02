DROP TABLE IF EXISTS webhook_deliveries;
ALTER TABLE webhooks DROP COLUMN IF EXISTS secret_encrypted;
ALTER TABLE social_accounts
    DROP COLUMN IF EXISTS refresh_token_encrypted,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS external_identifier,
    DROP COLUMN IF EXISTS token_scope;
DROP TABLE IF EXISTS oauth_states;
