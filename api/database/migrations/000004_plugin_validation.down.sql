ALTER TABLE plugins
  DROP COLUMN IF EXISTS validation_checks,
  DROP COLUMN IF EXISTS validated_at;
