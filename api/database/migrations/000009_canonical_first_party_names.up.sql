-- Corrected before migration 000009's first production rollout from version 8.
-- Databases already recorded at version 9 will not rerun this file; any such
-- pre-release environment needs the equivalent function replacement or rebuild.

CREATE TABLE IF NOT EXISTS plugin_aliases (
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    alias VARCHAR(356) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plugin_id, alias)
);

CREATE OR REPLACE FUNCTION merge_semrel_plugin_duplicate(source_plugin_id INTEGER, target_plugin_id INTEGER)
RETURNS VOID AS $$
DECLARE
    source_version RECORD;
    target_version RECORD;
    source_aliases TEXT[];
    source_alias TEXT;
BEGIN
    IF source_plugin_id = target_plugin_id THEN
        RETURN;
    END IF;

    PERFORM id
    FROM plugins
    WHERE id IN (source_plugin_id, target_plugin_id)
    ORDER BY id
    FOR UPDATE;

    IF NOT EXISTS (SELECT 1 FROM plugins WHERE id = source_plugin_id)
       OR NOT EXISTS (SELECT 1 FROM plugins WHERE id = target_plugin_id) THEN
        RAISE EXCEPTION 'cannot merge plugins % into %: plugin is missing',
            source_plugin_id, target_plugin_id;
    END IF;

    SELECT ARRAY_AGG(alias ORDER BY alias) INTO source_aliases
    FROM plugin_aliases
    WHERE plugin_id = source_plugin_id;
    DELETE FROM plugin_aliases WHERE plugin_id = source_plugin_id;

    -- The caller deterministically selects the canonical target. Description,
    -- author, license, and status are non-artifact display/workflow metadata:
    -- keep nonempty target fields, fill only empty fields from the source, and
    -- leave target status unchanged. Artifact conflicts remain fatal below.
    UPDATE plugins target
    SET views = target.views + source.views,
        downloads = target.downloads + source.downloads,
        tags = (
            SELECT COALESCE(ARRAY_AGG(DISTINCT tag ORDER BY tag), ARRAY[]::TEXT[])
            FROM UNNEST(target.tags || source.tags) AS tag
        ),
        description = COALESCE(NULLIF(target.description, ''), source.description),
        author = COALESCE(NULLIF(target.author, ''), source.author),
        license = COALESCE(NULLIF(target.license, ''), source.license),
        validation_checks = CASE
            WHEN source.validated_at IS NOT NULL
                 AND (target.validated_at IS NULL OR source.validated_at > target.validated_at)
                THEN source.validation_checks
            ELSE COALESCE(target.validation_checks, source.validation_checks)
        END,
        validated_at = CASE
            WHEN target.validated_at IS NULL THEN source.validated_at
            WHEN source.validated_at IS NULL THEN target.validated_at
            ELSE GREATEST(target.validated_at, source.validated_at)
        END,
        updated_at = GREATEST(target.updated_at, source.updated_at)
    FROM plugins source
    WHERE target.id = target_plugin_id
      AND source.id = source_plugin_id;

    FOR source_version IN
        SELECT *
        FROM plugin_versions
        WHERE plugin_id = source_plugin_id
        ORDER BY id
        FOR UPDATE
    LOOP
        SELECT * INTO target_version
        FROM plugin_versions
        WHERE plugin_id = target_plugin_id
          AND version = source_version.version
        FOR UPDATE;

        IF FOUND THEN
            IF target_version.release_date IS DISTINCT FROM source_version.release_date
               OR target_version.changelog IS DISTINCT FROM source_version.changelog
               OR target_version.download_url IS DISTINCT FROM source_version.download_url
               OR COALESCE(target_version.prerelease, FALSE)
                  <> COALESCE(source_version.prerelease, FALSE) THEN
                RAISE EXCEPTION
                    'cannot merge duplicate first-party version %: version metadata differs',
                    source_version.version;
            END IF;

            IF EXISTS (
                SELECT 1
                FROM plugin_checksums
                WHERE version_id IN (source_version.id, target_version.id)
                GROUP BY LOWER(platform)
                HAVING COUNT(DISTINCT (algorithm, hash)) > 1
            ) THEN
                RAISE EXCEPTION
                    'cannot merge duplicate first-party version %: checksums differ',
                    source_version.version;
            END IF;

            UPDATE plugin_versions
            SET views = views + source_version.views,
                downloads = downloads + source_version.downloads
            WHERE id = target_version.id;

            DELETE FROM plugin_checksums source_checksum
            USING plugin_checksums target_checksum
            WHERE source_checksum.version_id = source_version.id
              AND target_checksum.version_id = target_version.id
              AND LOWER(source_checksum.platform) = LOWER(target_checksum.platform)
              AND source_checksum.algorithm = target_checksum.algorithm
              AND source_checksum.hash = target_checksum.hash;

            UPDATE plugin_checksums
            SET version_id = target_version.id
            WHERE version_id = source_version.id;

            DELETE FROM plugin_checksums duplicate
            USING plugin_checksums retained
            WHERE duplicate.version_id = target_version.id
              AND retained.version_id = target_version.id
              AND duplicate.id > retained.id
              AND LOWER(duplicate.platform) = LOWER(retained.platform)
              AND duplicate.algorithm = retained.algorithm
              AND duplicate.hash = retained.hash;

            UPDATE metric_events
            SET version_id = target_version.id
            WHERE version_id = source_version.id;

            INSERT INTO metric_daily_version
                (day, plugin_id, version_id, metric_type, count, updated_at)
            SELECT day, target_plugin_id, target_version.id,
                   metric_type, count, updated_at
            FROM metric_daily_version
            WHERE version_id = source_version.id
            ON CONFLICT (day, version_id, metric_type) DO UPDATE
            SET count = metric_daily_version.count + EXCLUDED.count,
                updated_at = GREATEST(metric_daily_version.updated_at, EXCLUDED.updated_at);

            DELETE FROM metric_daily_version WHERE version_id = source_version.id;
            DELETE FROM plugin_versions WHERE id = source_version.id;
        ELSE
            UPDATE plugin_versions
            SET plugin_id = target_plugin_id
            WHERE id = source_version.id;
            UPDATE metric_daily_version
            SET plugin_id = target_plugin_id
            WHERE version_id = source_version.id;
        END IF;
    END LOOP;

    UPDATE metric_events
    SET plugin_id = target_plugin_id
    WHERE plugin_id = source_plugin_id;

    INSERT INTO metric_daily_plugin (day, plugin_id, metric_type, count, updated_at)
    SELECT day, target_plugin_id, metric_type, count, updated_at
    FROM metric_daily_plugin
    WHERE plugin_id = source_plugin_id
    ON CONFLICT (day, plugin_id, metric_type) DO UPDATE
    SET count = metric_daily_plugin.count + EXCLUDED.count,
        updated_at = GREATEST(metric_daily_plugin.updated_at, EXCLUDED.updated_at);

    DELETE FROM metric_daily_plugin WHERE plugin_id = source_plugin_id;
    DELETE FROM plugins WHERE id = source_plugin_id;

    FOREACH source_alias IN ARRAY COALESCE(source_aliases, ARRAY[]::TEXT[])
    LOOP
        INSERT INTO plugin_aliases (plugin_id, alias)
        SELECT target_plugin_id, source_alias
        WHERE NOT EXISTS (
            SELECT 1 FROM plugin_aliases WHERE LOWER(alias) = LOWER(source_alias)
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE TEMP TABLE semrel_first_party_names (
    target_name TEXT PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO semrel_first_party_names (target_name) VALUES
    ('analyzer-conventional'), ('analyzer-default'),
    ('condition-bitbucket-pipelines'), ('condition-circleci'), ('condition-generic'),
    ('condition-gitea-actions'), ('condition-github-actions'), ('condition-gitlab-ci'),
    ('generator-changelog-html'), ('generator-changelog-md'), ('generator-release-notes'),
    ('hook-discord'), ('hook-email'), ('hook-gitplugin'), ('hook-jira'), ('hook-matrix'),
    ('hook-slack'), ('hook-teams'), ('packager-nfpm'), ('provider-bitbucket'),
    ('provider-git'), ('provider-gitea'), ('provider-github'), ('provider-gitlab'),
    ('publisher-crates'), ('publisher-generic-http'), ('publisher-npm'), ('publisher-oci'),
    ('publisher-pypi'), ('updater-cargo'), ('updater-composer'), ('updater-docker'),
    ('updater-go'), ('updater-gradle'), ('updater-helm'), ('updater-homebrew'),
    ('updater-maven'), ('updater-npm'), ('updater-nuget'), ('updater-pubspec'),
    ('updater-python'), ('updater-terraform');

CREATE TEMP TABLE semrel_canonical_candidates ON COMMIT DROP AS
SELECT plugin.id,
       LOWER(regexp_replace(plugin.repository, '\.git$', '', 'i')) AS repository_key,
       first_party.target_name
FROM plugins plugin
JOIN semrel_first_party_names first_party
  ON LOWER(regexp_replace(plugin.repository, '\.git$', '', 'i'))
     = 'https://github.com/semrels/' || first_party.target_name
WHERE plugin.deleted_at IS NULL;

CREATE TEMP TABLE semrel_canonical_duplicate_map ON COMMIT DROP AS
WITH ranked AS (
    -- Canonical precedence is exact typed identity, then scoped legacy identity,
    -- then oldest ID. This makes every source merge deterministic.
    SELECT c.id,
           FIRST_VALUE(c.id) OVER (
               PARTITION BY c.repository_key
               ORDER BY (LOWER(COALESCE(p.namespace, '')) = '@semrel'
                         AND LOWER(p.name) = LOWER(c.target_name)) DESC,
                        (LOWER(COALESCE(p.namespace, '')) = '@semrel') DESC,
                        c.id
           ) AS target_id,
           ROW_NUMBER() OVER (
               PARTITION BY c.repository_key
               ORDER BY (LOWER(COALESCE(p.namespace, '')) = '@semrel'
                         AND LOWER(p.name) = LOWER(c.target_name)) DESC,
                        (LOWER(COALESCE(p.namespace, '')) = '@semrel') DESC,
                        c.id
           ) AS position
    FROM semrel_canonical_candidates c
    JOIN plugins p ON p.id = c.id
)
SELECT id AS source_id, target_id
FROM ranked
WHERE position > 1;

CREATE TEMP TABLE semrel_canonical_version_map ON COMMIT DROP AS
WITH version_candidates AS (
    SELECT version.id,
           version.plugin_id,
           version.version,
           COALESCE(duplicate.target_id, version.plugin_id) AS target_plugin_id
    FROM plugin_versions version
    JOIN semrel_canonical_candidates candidate ON candidate.id = version.plugin_id
    LEFT JOIN semrel_canonical_duplicate_map duplicate ON duplicate.source_id = version.plugin_id
),
ranked AS (
    SELECT candidate.*,
           FIRST_VALUE(candidate.id) OVER (
               PARTITION BY candidate.target_plugin_id, candidate.version
               ORDER BY (candidate.plugin_id = candidate.target_plugin_id) DESC, candidate.id
           ) AS target_version_id,
           ROW_NUMBER() OVER (
               PARTITION BY candidate.target_plugin_id, candidate.version
               ORDER BY (candidate.plugin_id = candidate.target_plugin_id) DESC, candidate.id
           ) AS position
    FROM version_candidates candidate
)
SELECT id AS source_version_id,
       target_version_id,
       plugin_id AS source_id,
       target_plugin_id AS target_id
FROM ranked
WHERE position > 1;

DO $$
DECLARE
    collision TEXT;
BEGIN
    SELECT c.target_name INTO collision
    FROM semrel_canonical_candidates c
    JOIN plugins occupied
      ON occupied.id <> c.id
     AND LOWER(COALESCE(occupied.namespace, '')) = '@semrel'
     AND LOWER(occupied.name) = LOWER(c.target_name)
    LEFT JOIN semrel_canonical_candidates occupied_candidate ON occupied_candidate.id = occupied.id
    WHERE occupied_candidate.repository_key IS NULL
       OR occupied_candidate.repository_key <> c.repository_key
    LIMIT 1;

    IF collision IS NOT NULL THEN
        RAISE EXCEPTION 'cannot canonicalize first-party plugin %: canonical identity is already occupied', collision;
    END IF;

    SELECT source_version.version INTO collision
    FROM semrel_canonical_version_map version_map
    JOIN plugin_versions source_version ON source_version.id = version_map.source_version_id
    JOIN plugin_versions target_version ON target_version.id = version_map.target_version_id
    WHERE target_version.release_date IS DISTINCT FROM source_version.release_date
       OR target_version.changelog IS DISTINCT FROM source_version.changelog
       OR target_version.download_url IS DISTINCT FROM source_version.download_url
       OR COALESCE(target_version.prerelease, FALSE) <> COALESCE(source_version.prerelease, FALSE)
    LIMIT 1;

    IF collision IS NOT NULL THEN
        RAISE EXCEPTION 'cannot merge duplicate first-party version %: version metadata differs', collision;
    END IF;
END $$;

DO $$
DECLARE
    duplicate RECORD;
BEGIN
    FOR duplicate IN
        SELECT source_id, target_id
        FROM semrel_canonical_duplicate_map
        ORDER BY target_id, source_id
    LOOP
        PERFORM merge_semrel_plugin_duplicate(duplicate.source_id, duplicate.target_id);
    END LOOP;
END $$;

UPDATE plugins
SET namespace = '@semrel',
    name = candidate.target_name,
    category = split_part(candidate.target_name, '-', 1),
    updated_at = NOW()
FROM semrel_canonical_candidates candidate
WHERE plugins.id = candidate.id
  AND plugins.deleted_at IS NULL;

DO $$
DECLARE
    collision TEXT;
BEGIN
    WITH desired(plugin_id, alias) AS (
        SELECT p.id, p.name
        FROM plugins p
        JOIN semrel_canonical_candidates candidate ON candidate.id = p.id
        WHERE p.deleted_at IS NULL
          AND p.namespace = '@semrel'
        UNION ALL
        SELECT p.id, legacy.alias
        FROM plugins p
        JOIN (
            VALUES
                ('analyzer-conventional', 'conventional'), ('analyzer-default', 'default'),
                ('condition-bitbucket-pipelines', 'bitbucket-pipelines'), ('condition-circleci', 'circleci'),
                ('condition-generic', 'generic'), ('condition-gitea-actions', 'gitea-actions'),
                ('condition-github-actions', 'github-actions'), ('condition-gitlab-ci', 'gitlab-ci'),
                ('generator-changelog-html', 'changelog-html'), ('generator-changelog-md', 'changelog-md'),
                ('generator-release-notes', 'release-notes'), ('hook-discord', 'discord'),
                ('hook-email', 'email'), ('hook-gitplugin', 'gitplugin'), ('hook-jira', 'jira'),
                ('hook-matrix', 'matrix'), ('hook-slack', 'slack'), ('hook-teams', 'teams'),
                ('packager-nfpm', 'nfpm'), ('provider-bitbucket', 'bitbucket'), ('provider-git', 'git'),
                ('provider-gitea', 'gitea'), ('provider-github', 'github'), ('provider-gitlab', 'gitlab'),
                ('publisher-crates', 'crates'), ('publisher-generic-http', 'generic-http'),
                ('publisher-oci', 'oci'), ('publisher-pypi', 'pypi'), ('updater-cargo', 'cargo'),
                ('updater-composer', 'composer'), ('updater-docker', 'docker'), ('updater-go', 'go'),
                ('updater-gradle', 'gradle'), ('updater-helm', 'helm'), ('updater-homebrew', 'homebrew'),
                ('updater-maven', 'maven'), ('updater-npm', 'npm'), ('updater-nuget', 'nuget'),
                ('updater-pubspec', 'pubspec'), ('updater-python', 'python'),
                ('updater-terraform', 'terraform')
        ) AS legacy(target, alias) ON legacy.target = p.name
        JOIN semrel_canonical_candidates candidate ON candidate.id = p.id
        WHERE p.namespace = '@semrel' AND p.deleted_at IS NULL
        UNION ALL
        SELECT p.id, '@semrel/' || legacy.alias
        FROM plugins p
        JOIN (
            VALUES
                ('analyzer-conventional', 'conventional'), ('analyzer-default', 'default'),
                ('condition-bitbucket-pipelines', 'bitbucket-pipelines'), ('condition-circleci', 'circleci'),
                ('condition-generic', 'generic'), ('condition-gitea-actions', 'gitea-actions'),
                ('condition-github-actions', 'github-actions'), ('condition-gitlab-ci', 'gitlab-ci'),
                ('generator-changelog-html', 'changelog-html'), ('generator-changelog-md', 'changelog-md'),
                ('generator-release-notes', 'release-notes'), ('hook-discord', 'discord'),
                ('hook-email', 'email'), ('hook-gitplugin', 'gitplugin'), ('hook-jira', 'jira'),
                ('hook-matrix', 'matrix'), ('hook-slack', 'slack'), ('hook-teams', 'teams'),
                ('packager-nfpm', 'nfpm'), ('provider-bitbucket', 'bitbucket'), ('provider-git', 'git'),
                ('provider-gitea', 'gitea'), ('provider-github', 'github'), ('provider-gitlab', 'gitlab'),
                ('publisher-crates', 'crates'), ('publisher-generic-http', 'generic-http'),
                ('publisher-oci', 'oci'), ('publisher-pypi', 'pypi'), ('updater-cargo', 'cargo'),
                ('updater-composer', 'composer'), ('updater-docker', 'docker'), ('updater-go', 'go'),
                ('updater-gradle', 'gradle'), ('updater-helm', 'helm'), ('updater-homebrew', 'homebrew'),
                ('updater-maven', 'maven'), ('updater-npm', 'npm'), ('updater-nuget', 'nuget'),
                ('updater-pubspec', 'pubspec'), ('updater-python', 'python'),
                ('updater-terraform', 'terraform')
        ) AS legacy(target, alias) ON legacy.target = p.name
        JOIN semrel_canonical_candidates candidate ON candidate.id = p.id
        WHERE p.namespace = '@semrel' AND p.deleted_at IS NULL
    )
    SELECT alias INTO collision
    FROM (
        SELECT d.alias
        FROM desired d
        JOIN plugin_aliases existing ON LOWER(existing.alias) = LOWER(d.alias)
        WHERE existing.plugin_id <> d.plugin_id
        UNION ALL
        SELECT d.alias
        FROM desired d
        JOIN plugins existing
          ON existing.deleted_at IS NULL
         AND existing.id <> d.plugin_id
         AND LOWER(CASE WHEN existing.namespace IS NULL OR existing.namespace = ''
                        THEN existing.name ELSE existing.namespace || '/' || existing.name END) = LOWER(d.alias)
    ) conflicts
    LIMIT 1;

    IF collision IS NOT NULL THEN
        RAISE EXCEPTION 'cannot install first-party alias %: it is already owned by another plugin', collision;
    END IF;
END $$;

INSERT INTO plugin_aliases (plugin_id, alias)
SELECT p.id, p.name
FROM plugins p
JOIN semrel_canonical_candidates candidate ON candidate.id = p.id
WHERE p.deleted_at IS NULL
  AND p.namespace = '@semrel'
ON CONFLICT DO NOTHING;

INSERT INTO plugin_aliases (plugin_id, alias)
SELECT p.id, aliases.alias
FROM plugins p
JOIN (
    VALUES
        ('analyzer-conventional', 'conventional'), ('analyzer-default', 'default'),
        ('condition-bitbucket-pipelines', 'bitbucket-pipelines'), ('condition-circleci', 'circleci'),
        ('condition-generic', 'generic'), ('condition-gitea-actions', 'gitea-actions'),
        ('condition-github-actions', 'github-actions'), ('condition-gitlab-ci', 'gitlab-ci'),
        ('generator-changelog-html', 'changelog-html'), ('generator-changelog-md', 'changelog-md'),
        ('generator-release-notes', 'release-notes'), ('hook-discord', 'discord'),
        ('hook-email', 'email'), ('hook-gitplugin', 'gitplugin'), ('hook-jira', 'jira'),
        ('hook-matrix', 'matrix'), ('hook-slack', 'slack'), ('hook-teams', 'teams'),
        ('packager-nfpm', 'nfpm'), ('provider-bitbucket', 'bitbucket'), ('provider-git', 'git'),
        ('provider-gitea', 'gitea'), ('provider-github', 'github'), ('provider-gitlab', 'gitlab'),
        ('publisher-crates', 'crates'), ('publisher-generic-http', 'generic-http'),
        ('publisher-oci', 'oci'), ('publisher-pypi', 'pypi'), ('updater-cargo', 'cargo'),
        ('updater-composer', 'composer'), ('updater-docker', 'docker'), ('updater-go', 'go'),
        ('updater-gradle', 'gradle'), ('updater-helm', 'helm'), ('updater-homebrew', 'homebrew'),
        ('updater-maven', 'maven'), ('updater-npm', 'npm'), ('updater-nuget', 'nuget'),
        ('updater-pubspec', 'pubspec'), ('updater-python', 'python'),
        ('updater-terraform', 'terraform')
) AS legacy(target, short) ON legacy.target = p.name
JOIN semrel_canonical_candidates candidate ON candidate.id = p.id
CROSS JOIN LATERAL (VALUES (legacy.short), ('@semrel/' || legacy.short)) AS aliases(alias)
WHERE p.namespace = '@semrel' AND p.deleted_at IS NULL
ON CONFLICT DO NOTHING;

CREATE TABLE plugin_identity_claims (
    normalized_ref TEXT PRIMARY KEY,
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION claim_plugin_identity(ref TEXT, owner_id INTEGER) RETURNS VOID AS $$
DECLARE
    existing_owner INTEGER;
BEGIN
    INSERT INTO plugin_identity_claims (normalized_ref, plugin_id)
    VALUES (LOWER(ref), owner_id)
    ON CONFLICT (normalized_ref) DO NOTHING;

    SELECT plugin_id INTO existing_owner
    FROM plugin_identity_claims
    WHERE normalized_ref = LOWER(ref);

    IF existing_owner <> owner_id THEN
        RAISE EXCEPTION 'plugin identity % is already owned by plugin %', ref, existing_owner;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION refresh_plugin_identity_claim(ref TEXT, owner_id INTEGER) RETURNS VOID AS $$
BEGIN
    DELETE FROM plugin_identity_claims
    WHERE normalized_ref = LOWER(ref) AND plugin_id = owner_id;

    IF EXISTS (
        SELECT 1
        FROM plugins p
        WHERE p.id = owner_id
          AND p.deleted_at IS NULL
          AND LOWER(CASE WHEN p.namespace IS NULL OR p.namespace = ''
                         THEN p.name ELSE p.namespace || '/' || p.name END) = LOWER(ref)
        UNION ALL
        SELECT 1
        FROM plugin_aliases a
        JOIN plugins p ON p.id = a.plugin_id AND p.deleted_at IS NULL
        WHERE a.plugin_id = owner_id AND LOWER(a.alias) = LOWER(ref)
    ) THEN
        PERFORM claim_plugin_identity(ref, owner_id);
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION maintain_plugin_canonical_claim() RETURNS trigger AS $$
DECLARE
    old_ref TEXT;
    new_ref TEXT;
    alias_ref RECORD;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;

    IF TG_OP = 'UPDATE' THEN
        old_ref := CASE WHEN OLD.namespace IS NULL OR OLD.namespace = ''
                        THEN OLD.name ELSE OLD.namespace || '/' || OLD.name END;
        PERFORM refresh_plugin_identity_claim(old_ref, OLD.id);
    END IF;

    IF NEW.deleted_at IS NULL THEN
        new_ref := CASE WHEN NEW.namespace IS NULL OR NEW.namespace = ''
                        THEN NEW.name ELSE NEW.namespace || '/' || NEW.name END;
        PERFORM claim_plugin_identity(new_ref, NEW.id);
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.deleted_at IS DISTINCT FROM NEW.deleted_at THEN
        FOR alias_ref IN
            SELECT alias FROM plugin_aliases WHERE plugin_id = NEW.id ORDER BY LOWER(alias)
        LOOP
            PERFORM refresh_plugin_identity_claim(alias_ref.alias, NEW.id);
        END LOOP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION maintain_plugin_alias_claim() RETURNS trigger AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        PERFORM refresh_plugin_identity_claim(OLD.alias, OLD.plugin_id);
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') AND EXISTS (
        SELECT 1 FROM plugins WHERE id = NEW.plugin_id AND deleted_at IS NULL
    ) THEN
        PERFORM claim_plugin_identity(NEW.alias, NEW.plugin_id);
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

INSERT INTO plugin_identity_claims (normalized_ref, plugin_id)
SELECT LOWER(CASE WHEN namespace IS NULL OR namespace = ''
                  THEN name ELSE namespace || '/' || name END), id
FROM plugins
WHERE deleted_at IS NULL;

DO $$
DECLARE
    alias_claim RECORD;
BEGIN
    FOR alias_claim IN
        SELECT a.alias, a.plugin_id
        FROM plugin_aliases a
        JOIN plugins p ON p.id = a.plugin_id AND p.deleted_at IS NULL
        ORDER BY LOWER(a.alias), a.plugin_id
    LOOP
        PERFORM claim_plugin_identity(alias_claim.alias, alias_claim.plugin_id);
    END LOOP;
END $$;

CREATE TRIGGER plugin_identity_claim_guard
AFTER INSERT OR UPDATE OF namespace, name, deleted_at ON plugins
FOR EACH ROW EXECUTE FUNCTION maintain_plugin_canonical_claim();

CREATE TRIGGER plugin_alias_claim_guard
AFTER INSERT OR UPDATE OR DELETE ON plugin_aliases
FOR EACH ROW EXECUTE FUNCTION maintain_plugin_alias_claim();

DROP INDEX IF EXISTS plugin_aliases_lookup_uq;

CREATE OR REPLACE FUNCTION lock_semrel_plugin_write() RETURNS trigger AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(91557115086156);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS plugin_write_migration_guard ON plugins;
CREATE TRIGGER plugin_write_migration_guard
BEFORE INSERT OR UPDATE OR DELETE ON plugins
FOR EACH STATEMENT EXECUTE FUNCTION lock_semrel_plugin_write();

DROP TRIGGER IF EXISTS plugin_alias_write_migration_guard ON plugin_aliases;
CREATE TRIGGER plugin_alias_write_migration_guard
BEFORE INSERT OR UPDATE OR DELETE ON plugin_aliases
FOR EACH STATEMENT EXECUTE FUNCTION lock_semrel_plugin_write();
