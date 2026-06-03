-- There is no safe way to reverse the deduplication because the deleted rows
-- had no canonical data that is not already present in the namespaced rows.
-- This migration is intentionally irreversible.
SELECT 1;
