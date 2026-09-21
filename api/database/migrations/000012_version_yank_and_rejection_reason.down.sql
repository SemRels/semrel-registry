DROP INDEX IF EXISTS idx_plugin_versions_yanked_at;

ALTER TABLE plugin_versions
  DROP COLUMN IF EXISTS yanked_at,
  DROP COLUMN IF EXISTS yanked_by,
  DROP COLUMN IF EXISTS yanked_reason;

ALTER TABLE plugins
  DROP COLUMN IF EXISTS rejection_reason,
  DROP COLUMN IF EXISTS reviewed_at,
  DROP COLUMN IF EXISTS reviewed_by;
