ALTER TABLE plugins
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS deletion_reason TEXT,
    ADD COLUMN IF NOT EXISTS deletion_confirmation TEXT;

ALTER TABLE plugin_versions
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS deletion_reason TEXT,
    ADD COLUMN IF NOT EXISTS deletion_confirmation TEXT;

CREATE INDEX IF NOT EXISTS idx_plugin_versions_active
    ON plugin_versions (plugin_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS account_deletions (
    id SERIAL PRIMARY KEY,
    login VARCHAR(255) NOT NULL,
    deleted_by VARCHAR(255) NOT NULL,
    reason TEXT,
    confirmation TEXT NOT NULL,
    plugins_deleted INT NOT NULL DEFAULT 0,
    versions_deleted INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
