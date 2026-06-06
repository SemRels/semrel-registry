DROP INDEX IF EXISTS idx_metric_daily_version_metric_day;
DROP INDEX IF EXISTS idx_metric_daily_plugin_metric_day;
DROP INDEX IF EXISTS idx_metric_events_occurred_at;
DROP INDEX IF EXISTS idx_metric_events_metric_type;
DROP INDEX IF EXISTS idx_metric_events_version_id;
DROP INDEX IF EXISTS idx_metric_events_plugin_id;

DROP TABLE IF EXISTS metric_daily;
DROP TABLE IF EXISTS metric_events;

ALTER TABLE plugin_versions
    DROP COLUMN IF EXISTS downloads,
    DROP COLUMN IF EXISTS views;

ALTER TABLE plugins
    DROP COLUMN IF EXISTS downloads,
    DROP COLUMN IF EXISTS views;
