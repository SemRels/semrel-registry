CREATE TABLE webhook_subscriptions (
  id                   BIGSERIAL PRIMARY KEY,
  created_by           TEXT NOT NULL,
  url                  TEXT NOT NULL,
  secret               TEXT NOT NULL,
  plugin_ref           TEXT NOT NULL,
  events               TEXT[] NOT NULL,
  active               BOOLEAN NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_delivery_at     TIMESTAMPTZ,
  last_status_code     INTEGER,
  consecutive_failures INTEGER NOT NULL DEFAULT 0
);

-- Looked up on every delivery: which active subscriptions want this plugin.
CREATE INDEX idx_webhook_subscriptions_delivery
  ON webhook_subscriptions (plugin_ref)
  WHERE active;

CREATE INDEX idx_webhook_subscriptions_created_by
  ON webhook_subscriptions (created_by);
