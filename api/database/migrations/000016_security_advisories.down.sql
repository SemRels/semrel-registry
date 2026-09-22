DROP INDEX IF EXISTS idx_plugins_security_advisories;

ALTER TABLE plugins
  DROP COLUMN IF EXISTS security_advisories,
  DROP COLUMN IF EXISTS advisories_checked_at;
