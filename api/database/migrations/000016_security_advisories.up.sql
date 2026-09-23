ALTER TABLE plugins
  ADD COLUMN security_advisories   JSONB,
  ADD COLUMN advisories_checked_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_plugins_security_advisories
  ON plugins (id)
  WHERE security_advisories IS NOT NULL;
