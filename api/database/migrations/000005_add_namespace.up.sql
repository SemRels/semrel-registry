-- Add namespace column for scoped plugin identifiers (e.g. @semrel, @google).
ALTER TABLE plugins ADD COLUMN namespace VARCHAR(100);

-- Drop the old global unique constraint on name so that different namespaces
-- can each have a plugin with the same short name.
ALTER TABLE plugins DROP CONSTRAINT plugins_name_key;

-- Unnamespaced plugins: name must still be globally unique.
CREATE UNIQUE INDEX plugins_name_unscoped_uq
    ON plugins(name)
    WHERE namespace IS NULL;

-- Namespaced plugins: the (namespace, name) pair must be unique.
CREATE UNIQUE INDEX plugins_namespace_name_uq
    ON plugins(namespace, name)
    WHERE namespace IS NOT NULL;
