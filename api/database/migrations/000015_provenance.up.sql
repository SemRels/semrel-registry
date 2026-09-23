-- Build provenance for a published artifact.
--
-- A checksum proves the bytes did not change in transit. It says nothing about
-- where they came from: a publisher with a stolen token can compute a correct
-- checksum for a malicious binary. Provenance answers the other question —
-- which repository and which workflow produced this artifact — by checking the
-- attestation GitHub records at build time against the digest the registry
-- already stores.
ALTER TABLE plugin_versions
  ADD COLUMN provenance          JSONB,
  ADD COLUMN provenance_checked_at TIMESTAMPTZ;

-- The catalogue endpoint surfaces provenance for every version it returns.
CREATE INDEX IF NOT EXISTS idx_plugin_versions_provenance
  ON plugin_versions (plugin_id)
  WHERE provenance IS NOT NULL;
