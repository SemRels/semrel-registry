-- Add status column for community plugin submission workflow.
-- active   = published and visible in public registry
-- pending  = submitted by community user, awaiting admin review
-- rejected = rejected by admin (hidden from public, visible to submitter)

ALTER TABLE plugins
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'pending', 'rejected'));

CREATE INDEX IF NOT EXISTS idx_plugins_status ON plugins (status);
