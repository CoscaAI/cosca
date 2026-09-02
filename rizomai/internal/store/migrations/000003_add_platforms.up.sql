-- ============================================================================
-- RIZOMAI — migration 000003: EXPANSÃO — novas plataformas no enum `platform`
-- (13 no total: x, linkedin, telegram, instagram, facebook, threads, youtube,
-- tiktok, bluesky, reddit, pinterest, snapchat, googlebusiness).
-- ============================================================================

ALTER TYPE platform ADD VALUE IF NOT EXISTS 'instagram';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'facebook';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'threads';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'youtube';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'tiktok';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'bluesky';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'reddit';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'pinterest';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'snapchat';
ALTER TYPE platform ADD VALUE IF NOT EXISTS 'googlebusiness';
