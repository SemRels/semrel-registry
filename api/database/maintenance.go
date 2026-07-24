package database

import (
	"context"
	"fmt"
	"time"

	"github.com/SemRels/semrel-registry/api/naming"
)

// CleanupSemrelDuplicates fixes historical duplicate rows created by older
// sync behavior for SemRels repositories. It is safe to run repeatedly.
func (d *Database) CleanupSemrelDuplicates(ctx context.Context) (deleted int64, normalized int64, err error) {
	if d == nil || d.pool == nil {
		return 0, 0, fmt.Errorf("database is not initialized")
	}

	txCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := d.pool.Begin(txCtx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin cleanup tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(txCtx)
		}
	}()
	if err = LockPluginWrites(txCtx, tx); err != nil {
		return 0, 0, err
	}

	_, err = tx.Exec(txCtx, `
		CREATE TEMP TABLE semrel_cleanup_targets (
			plugin_id BIGINT PRIMARY KEY,
			canonical_name TEXT NOT NULL,
			category TEXT NOT NULL,
			aliases TEXT[] NOT NULL
		) ON COMMIT DROP`)
	if err != nil {
		return 0, 0, fmt.Errorf("create cleanup targets: %w", err)
	}

	rows, err := tx.Query(txCtx, `
		SELECT id, repository
		FROM plugins
		WHERE deleted_at IS NULL
		FOR UPDATE`)
	if err != nil {
		return 0, 0, fmt.Errorf("lock cleanup candidates: %w", err)
	}
	type cleanupTarget struct {
		id        int64
		canonical naming.FirstPartyPlugin
	}
	var targets []cleanupTarget
	for rows.Next() {
		var id int64
		var repository *string
		if scanErr := rows.Scan(&id, &repository); scanErr != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan cleanup candidate: %w", scanErr)
		}
		if repository == nil {
			continue
		}
		if canonical, ok := naming.FirstPartyByRepositoryURL(*repository); ok {
			targets = append(targets, cleanupTarget{id: id, canonical: canonical})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, 0, fmt.Errorf("read cleanup candidates: %w", err)
	}
	rows.Close()

	for _, target := range targets {
		_, err = tx.Exec(txCtx, `
			INSERT INTO semrel_cleanup_targets (plugin_id, canonical_name, category, aliases)
			VALUES ($1, $2, $3, $4)`,
			target.id, target.canonical.Name, target.canonical.Category, target.canonical.Aliases)
		if err != nil {
			return 0, 0, fmt.Errorf("record cleanup target %d: %w", target.id, err)
		}
	}

	_, err = tx.Exec(txCtx, `
		CREATE TEMP TABLE semrel_dup_map ON COMMIT DROP AS
		WITH ranked AS (
			SELECT t.plugin_id,
			       FIRST_VALUE(t.plugin_id) OVER (
				       PARTITION BY t.canonical_name
				       ORDER BY (p.namespace = '@semrel' AND p.name = t.canonical_name) DESC,
				                (p.namespace = '@semrel') DESC,
				                t.plugin_id
			       ) AS target_id,
			       ROW_NUMBER() OVER (
				       PARTITION BY t.canonical_name
				       ORDER BY (p.namespace = '@semrel' AND p.name = t.canonical_name) DESC,
				                (p.namespace = '@semrel') DESC,
				                t.plugin_id
			       ) AS position
			FROM semrel_cleanup_targets t
			JOIN plugins p ON p.id = t.plugin_id
		)
		SELECT plugin_id AS src_id, target_id AS tgt_id
		FROM ranked
		WHERE position > 1`)
	if err != nil {
		return 0, 0, fmt.Errorf("build duplicate map: %w", err)
	}

	_, err = tx.Exec(txCtx, `
		CREATE TEMP TABLE semrel_desired_aliases ON COMMIT DROP AS
		SELECT DISTINCT owner_id, alias
		FROM (
			SELECT COALESCE(m.tgt_id, t.plugin_id) AS owner_id, alias
			FROM semrel_cleanup_targets t
			LEFT JOIN semrel_dup_map m ON m.src_id = t.plugin_id
			CROSS JOIN LATERAL UNNEST(t.aliases) AS alias
			UNION ALL
			SELECT m.tgt_id, a.alias
			FROM semrel_dup_map m
			JOIN plugin_aliases a ON a.plugin_id = m.src_id
		) desired;

		DO $$
		DECLARE collision TEXT;
		BEGIN
			SELECT canonical_name INTO collision
			FROM (
				SELECT t.canonical_name
				FROM semrel_cleanup_targets t
				JOIN plugins p
				  ON p.id <> t.plugin_id
				 AND LOWER(COALESCE(p.namespace, '')) = '@semrel'
				 AND LOWER(p.name) = LOWER(t.canonical_name)
				LEFT JOIN semrel_dup_map target_map ON target_map.src_id = t.plugin_id
				LEFT JOIN semrel_dup_map occupied_map ON occupied_map.src_id = p.id
				WHERE COALESCE(target_map.tgt_id, t.plugin_id)
				      <> COALESCE(occupied_map.tgt_id, p.id)
			) conflicts
			LIMIT 1;
			IF collision IS NOT NULL THEN
				RAISE EXCEPTION 'cannot normalize first-party plugin %: canonical identity is already occupied', collision;
			END IF;

			SELECT alias INTO collision
			FROM (
				SELECT MIN(alias) AS alias
				FROM semrel_desired_aliases
				GROUP BY LOWER(alias)
				HAVING COUNT(DISTINCT owner_id) > 1
				UNION ALL
				SELECT d.alias
				FROM semrel_desired_aliases d
				JOIN plugin_aliases a ON LOWER(a.alias) = LOWER(d.alias)
				LEFT JOIN semrel_dup_map m ON m.src_id = a.plugin_id
				WHERE COALESCE(m.tgt_id, a.plugin_id) <> d.owner_id
				UNION ALL
				SELECT d.alias
				FROM semrel_desired_aliases d
				JOIN plugins p
				  ON p.deleted_at IS NULL
				 AND LOWER(CASE WHEN p.namespace IS NULL OR p.namespace = ''
				                THEN p.name ELSE p.namespace || '/' || p.name END) = LOWER(d.alias)
				LEFT JOIN semrel_dup_map m ON m.src_id = p.id
				WHERE COALESCE(m.tgt_id, p.id) <> d.owner_id
			) conflicts
			LIMIT 1;
			IF collision IS NOT NULL THEN
				RAISE EXCEPTION 'cannot install first-party alias %: it is already owned by another plugin', collision;
			END IF;
		END $$`)
	if err != nil {
		return 0, 0, fmt.Errorf("validate canonical identities and aliases: %w", err)
	}

	if err = tx.QueryRow(txCtx, `SELECT COUNT(*) FROM semrel_dup_map`).Scan(&deleted); err != nil {
		return 0, 0, fmt.Errorf("count duplicate plugins: %w", err)
	}

	_, err = tx.Exec(txCtx, `
		DO $$
		DECLARE duplicate RECORD;
		BEGIN
			FOR duplicate IN
				SELECT src_id, tgt_id FROM semrel_dup_map ORDER BY tgt_id, src_id
			LOOP
				PERFORM merge_semrel_plugin_duplicate(
					duplicate.src_id::INTEGER,
					duplicate.tgt_id::INTEGER
				);
			END LOOP;
		END $$`)
	if err != nil {
		return 0, 0, fmt.Errorf("merge duplicate plugins: %w", err)
	}

	normalizedTag, err := tx.Exec(txCtx, `
		UPDATE plugins p
		SET    namespace  = '@semrel',
		       name       = target.canonical_name,
		       category   = target.category,
		       updated_at = NOW()
		FROM (
			SELECT DISTINCT COALESCE(m.tgt_id, t.plugin_id) AS plugin_id,
			                t.canonical_name,
			                t.category
			FROM semrel_cleanup_targets t
			LEFT JOIN semrel_dup_map m ON m.src_id = t.plugin_id
		) target
		WHERE p.id = target.plugin_id
		  AND (COALESCE(p.namespace, '') <> '@semrel'
		       OR p.name <> target.canonical_name
		       OR p.category <> target.category)`)
	if err != nil {
		return 0, 0, fmt.Errorf("normalize semrel names: %w", err)
	}

	_, err = tx.Exec(txCtx, `
		INSERT INTO plugin_aliases (plugin_id, alias)
		SELECT d.owner_id, d.alias
		FROM semrel_desired_aliases d
		WHERE NOT EXISTS (
			SELECT 1
			FROM plugin_aliases a
			WHERE LOWER(a.alias) = LOWER(d.alias)
		)`)
	if err != nil {
		return 0, 0, fmt.Errorf("install canonical aliases: %w", err)
	}

	if err = tx.Commit(txCtx); err != nil {
		return 0, 0, fmt.Errorf("commit cleanup tx: %w", err)
	}

	return deleted, normalizedTag.RowsAffected(), nil
}
