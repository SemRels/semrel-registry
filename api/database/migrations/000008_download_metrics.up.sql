ALTER TABLE plugins
    ADD COLUMN IF NOT EXISTS views BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS downloads BIGINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_versions
    ADD COLUMN IF NOT EXISTS views BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS downloads BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS metric_events (
    id BIGSERIAL PRIMARY KEY,
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    version_id INT REFERENCES plugin_versions(id) ON DELETE CASCADE,
    metric_type VARCHAR(16) NOT NULL,
    source VARCHAR(64) NOT NULL DEFAULT 'api',
    occurred_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT metric_events_metric_type_check CHECK (metric_type IN ('view', 'download'))
);

CREATE TABLE IF NOT EXISTS metric_daily_plugin (
    day DATE NOT NULL,
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    metric_type VARCHAR(16) NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT metric_daily_metric_type_check CHECK (metric_type IN ('view', 'download')),
    CONSTRAINT metric_daily_plugin_pk PRIMARY KEY (day, plugin_id, metric_type)
);

CREATE TABLE IF NOT EXISTS metric_daily_version (
    day DATE NOT NULL,
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    version_id INT NOT NULL REFERENCES plugin_versions(id) ON DELETE CASCADE,
    metric_type VARCHAR(16) NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT metric_daily_version_metric_type_check CHECK (metric_type IN ('view', 'download')),
    CONSTRAINT metric_daily_version_pk PRIMARY KEY (day, version_id, metric_type)
);

CREATE INDEX IF NOT EXISTS idx_metric_events_plugin_id ON metric_events(plugin_id);
CREATE INDEX IF NOT EXISTS idx_metric_events_version_id ON metric_events(version_id);
CREATE INDEX IF NOT EXISTS idx_metric_events_metric_type ON metric_events(metric_type);
CREATE INDEX IF NOT EXISTS idx_metric_events_occurred_at ON metric_events(occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_metric_daily_plugin_metric_day ON metric_daily_plugin(plugin_id, metric_type, day DESC);
CREATE INDEX IF NOT EXISTS idx_metric_daily_version_metric_day ON metric_daily_version(version_id, metric_type, day DESC);
