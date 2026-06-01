DROP INDEX IF EXISTS plugins_namespace_name_uq;
DROP INDEX IF EXISTS plugins_name_unscoped_uq;
ALTER TABLE plugins ADD CONSTRAINT plugins_name_key UNIQUE (name);
ALTER TABLE plugins DROP COLUMN namespace;
