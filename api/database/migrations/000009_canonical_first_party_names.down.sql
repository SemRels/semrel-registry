CREATE TEMP TABLE semrel_down_first_party (
    target_name TEXT PRIMARY KEY,
    legacy_name TEXT NOT NULL UNIQUE
) ON COMMIT DROP;

INSERT INTO semrel_down_first_party (target_name, legacy_name) VALUES
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
    ('publisher-npm', 'publisher-npm'), ('publisher-oci', 'oci'), ('publisher-pypi', 'pypi'),
    ('updater-cargo', 'cargo'), ('updater-composer', 'composer'), ('updater-docker', 'docker'),
    ('updater-go', 'go'), ('updater-gradle', 'gradle'), ('updater-helm', 'helm'),
    ('updater-homebrew', 'homebrew'), ('updater-maven', 'maven'), ('updater-npm', 'npm'),
    ('updater-nuget', 'nuget'), ('updater-pubspec', 'pubspec'),
    ('updater-python', 'python'), ('updater-terraform', 'terraform');

DO $$
DECLARE
    collision TEXT;
BEGIN
    SELECT first_party.legacy_name INTO collision
    FROM plugins
    JOIN semrel_down_first_party first_party
      ON first_party.target_name = plugins.name
     AND LOWER(regexp_replace(plugins.repository, '\.git$', '', 'i'))
         = 'https://github.com/semrels/' || first_party.target_name
    WHERE plugins.namespace = '@semrel'
      AND plugins.deleted_at IS NULL
    GROUP BY first_party.legacy_name
    HAVING COUNT(*) > 1
    LIMIT 1;

    IF collision IS NOT NULL THEN
        RAISE EXCEPTION 'cannot downgrade first-party names: short name % would collide', collision;
    END IF;
END $$;

DROP TRIGGER IF EXISTS plugin_alias_claim_guard ON plugin_aliases;
DROP TRIGGER IF EXISTS plugin_identity_claim_guard ON plugins;
DROP TRIGGER IF EXISTS plugin_alias_write_migration_guard ON plugin_aliases;
DROP TRIGGER IF EXISTS plugin_write_migration_guard ON plugins;
DROP FUNCTION IF EXISTS lock_semrel_plugin_write();
DROP FUNCTION IF EXISTS maintain_plugin_alias_claim();
DROP FUNCTION IF EXISTS maintain_plugin_canonical_claim();
DROP FUNCTION IF EXISTS refresh_plugin_identity_claim(TEXT, INTEGER);
DROP FUNCTION IF EXISTS claim_plugin_identity(TEXT, INTEGER);
DROP TABLE IF EXISTS plugin_identity_claims;
DROP FUNCTION IF EXISTS merge_semrel_plugin_duplicate(INTEGER, INTEGER);

UPDATE plugins
SET name = first_party.legacy_name,
    updated_at = NOW()
FROM semrel_down_first_party first_party
WHERE plugins.namespace = '@semrel'
  AND plugins.deleted_at IS NULL
  AND plugins.name = first_party.target_name
  AND LOWER(regexp_replace(plugins.repository, '\.git$', '', 'i'))
      = 'https://github.com/semrels/' || first_party.target_name;
