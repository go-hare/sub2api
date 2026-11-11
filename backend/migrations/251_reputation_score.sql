-- Reputation score routing: user integer score, account tier binding, score ledger.
-- Independent of content-moderation / risk-control settings.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS reputation_score INTEGER NOT NULL DEFAULT 60;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS reputation_tier_id VARCHAR(64);

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS reputation_score_enabled BOOLEAN;

CREATE TABLE IF NOT EXISTS reputation_score_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    event           VARCHAR(64) NOT NULL,
    delta           INTEGER NOT NULL,
    score_before    INTEGER NOT NULL,
    score_after     INTEGER NOT NULL,
    request_id      VARCHAR(128) NOT NULL DEFAULT '',
    account_id      BIGINT,
    amount          NUMERIC(20,8),
    actor_admin_id  BIGINT,
    reason          TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reputation_score_logs_user_created
    ON reputation_score_logs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_accounts_reputation_tier_id
    ON accounts (reputation_tier_id)
    WHERE reputation_tier_id IS NOT NULL AND deleted_at IS NULL;
