-- Backfill @semrel namespace for all existing plugins that have no namespace set.
-- All first-party SemRels plugins belong to the @semrel scope.
UPDATE plugins SET namespace = '@semrel' WHERE namespace IS NULL;
