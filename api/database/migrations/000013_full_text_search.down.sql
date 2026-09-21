DROP INDEX IF EXISTS idx_plugins_description_trgm;
DROP INDEX IF EXISTS idx_plugins_name_trgm;
DROP INDEX IF EXISTS idx_plugins_search_vector;

ALTER TABLE plugins
  DROP COLUMN IF EXISTS search_vector;

-- pg_trgm is left installed: other objects may depend on it, and dropping an
-- extension is not something a schema rollback should decide.
