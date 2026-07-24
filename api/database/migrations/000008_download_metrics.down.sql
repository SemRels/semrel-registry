DROP INDEX IF EXISTS idx_metric_daily_version_metric_day;
DROP INDEX IF EXISTS idx_metric_daily_plugin_metric_day;
DROP INDEX IF EXISTS idx_metric_events_occurred_at;
DROP INDEX IF EXISTS idx_metric_events_metric_type;
DROP INDEX IF EXISTS idx_metric_events_version_id;
DROP INDEX IF EXISTS idx_metric_events_plugin_id;

DROP TABLE IF EXISTS metric_daily_version;
DROP TABLE IF EXISTS metric_daily_plugin;
DROP TABLE IF EXISTS metric_events;

-- Migration 000009 deliberately retains aliases for a 9 -> 8 -> 9 round trip.
-- Once the rollback proceeds below version 8, remove that compatibility table
-- before migration 000001 drops its referenced plugins table.
DROP TABLE IF EXISTS plugin_aliases;

ALTER TABLE plugin_versions
    DROP COLUMN IF EXISTS downloads,
    DROP COLUMN IF EXISTS views;

ALTER TABLE plugins
    DROP COLUMN IF EXISTS downloads,
    DROP COLUMN IF EXISTS views;
