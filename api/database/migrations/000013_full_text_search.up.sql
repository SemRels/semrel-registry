-- Search was "ILIKE '%term%'" across four columns, which no index can serve:
-- every query was a sequential scan over the whole table, and results came back
-- in name order with no notion of relevance.

-- Trigram matching keeps substring search ("githu") working, but index-backed.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Weighted document: a name match should outrank a description mention.
ALTER TABLE plugins
  ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', coalesce(name, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(namespace, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(description, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(author, '')), 'C') ||
    setweight(to_tsvector('simple', coalesce(category, '')), 'C')
  ) STORED;

CREATE INDEX IF NOT EXISTS idx_plugins_search_vector
  ON plugins USING GIN (search_vector);

-- Plugin names are hyphenated identifiers ("provider-github"), which tokenise
-- poorly; trigrams cover the partial-word typing a search box actually sees.
CREATE INDEX IF NOT EXISTS idx_plugins_name_trgm
  ON plugins USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_plugins_description_trgm
  ON plugins USING GIN (description gin_trgm_ops);
