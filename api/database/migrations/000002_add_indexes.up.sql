CREATE INDEX IF NOT EXISTS idx_plugins_category ON plugins(category);
CREATE INDEX IF NOT EXISTS idx_plugins_name ON plugins(name);
CREATE INDEX IF NOT EXISTS idx_versions_plugin ON plugin_versions(plugin_id);
CREATE INDEX IF NOT EXISTS idx_checksums_version ON plugin_checksums(version_id);
