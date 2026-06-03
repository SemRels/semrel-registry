-- Remove bare-name duplicates that were created before the namespace fix.
--
-- After migration 000005/000006 backfilled namespace='@semrel' on existing rows,
-- a bug in SyncGitHubOrg could create a second, un-namespaced row for the same
-- plugin (namespace IS NULL or '') because the lookup used the bare name while
-- the existing row had a namespace prefix.  This migration deletes those stale
-- bare-name rows wherever a canonical namespaced counterpart already exists.
DELETE FROM plugins
WHERE (namespace IS NULL OR namespace = '')
  AND deleted_at IS NULL
  AND name IN (
      SELECT name
      FROM   plugins
      WHERE  namespace IS NOT NULL AND namespace != ''
        AND  deleted_at IS NULL
  );
