-- Yanking marks a published version as "do not use for new installs" without
-- removing it. Deleting a version breaks every build that already pins it;
-- yanking is how a registry retracts a compromised or broken release while
-- keeping existing lockfiles resolvable.
ALTER TABLE plugin_versions
  ADD COLUMN yanked_at     TIMESTAMPTZ,
  ADD COLUMN yanked_by     TEXT,
  ADD COLUMN yanked_reason TEXT;

-- Resolving "the latest version" must skip yanked releases, and the catalogue
-- endpoint filters on this column on every request.
CREATE INDEX IF NOT EXISTS idx_plugin_versions_yanked_at
  ON plugin_versions (plugin_id, yanked_at);

-- A rejected submission previously carried no explanation, leaving the author
-- with a "rejected" badge and nothing to act on.
ALTER TABLE plugins
  ADD COLUMN rejection_reason TEXT,
  ADD COLUMN reviewed_at      TIMESTAMPTZ,
  ADD COLUMN reviewed_by      TEXT;
