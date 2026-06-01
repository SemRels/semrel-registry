-- Revert: remove @semrel namespace that was backfilled (sets them back to NULL).
UPDATE plugins SET namespace = NULL WHERE namespace = '@semrel';
