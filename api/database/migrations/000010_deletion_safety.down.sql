DROP TABLE IF EXISTS account_deletions;

DROP INDEX IF EXISTS idx_plugin_versions_active;

ALTER TABLE plugin_versions
    DROP COLUMN IF EXISTS deletion_confirmation,
    DROP COLUMN IF EXISTS deletion_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE plugins
    DROP COLUMN IF EXISTS deletion_confirmation,
    DROP COLUMN IF EXISTS deletion_reason,
    DROP COLUMN IF EXISTS deleted_by;
