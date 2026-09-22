DROP INDEX IF EXISTS idx_plugin_versions_provenance;

ALTER TABLE plugin_versions
  DROP COLUMN IF EXISTS provenance,
  DROP COLUMN IF EXISTS provenance_checked_at;
