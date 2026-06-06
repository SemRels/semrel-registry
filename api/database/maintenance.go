package database

import (
	"context"
	"fmt"
	"time"
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

	_, err = tx.Exec(txCtx, `
		CREATE TEMP TABLE semrel_dup_map ON COMMIT DROP AS
		SELECT src.id AS src_id, tgt.id AS tgt_id
		FROM   plugins src
		JOIN   plugins tgt
		  ON   tgt.namespace = '@semrel'
		 AND   tgt.deleted_at IS NULL
		 AND   (
				tgt.name = src.name
				OR tgt.name = regexp_replace(src.name, '^(analyzer|condition|generator|hook|provider|updater)-', '')
		 )
		WHERE (src.namespace IS NULL OR src.namespace = '')
		  AND src.deleted_at IS NULL
		  AND src.repository LIKE 'https://github.com/SemRels/%'
		  AND src.id <> tgt.id`)
	if err != nil {
		return 0, 0, fmt.Errorf("build duplicate map: %w", err)
	}

	_, err = tx.Exec(txCtx, `
		INSERT INTO plugin_versions (plugin_id, version, release_date, changelog, download_url, prerelease, created_at)
		SELECT m.tgt_id, v.version, v.release_date, v.changelog, v.download_url, v.prerelease, v.created_at
		FROM semrel_dup_map m
		JOIN plugin_versions v ON v.plugin_id = m.src_id
		ON CONFLICT (plugin_id, version) DO NOTHING`)
	if err != nil {
		return 0, 0, fmt.Errorf("merge duplicate versions: %w", err)
	}

	_, err = tx.Exec(txCtx, `
		INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
		SELECT tv.id, c.platform, c.algorithm, c.hash
		FROM semrel_dup_map m
		JOIN plugin_versions sv ON sv.plugin_id = m.src_id
		JOIN plugin_checksums c ON c.version_id = sv.id
		JOIN plugin_versions tv ON tv.plugin_id = m.tgt_id AND tv.version = sv.version
		LEFT JOIN plugin_checksums ec
		  ON ec.version_id = tv.id
		 AND ec.platform = c.platform
		 AND ec.algorithm = c.algorithm
		 AND ec.hash = c.hash
		WHERE ec.id IS NULL`)
	if err != nil {
		return 0, 0, fmt.Errorf("merge duplicate checksums: %w", err)
	}

	deletedTag, err := tx.Exec(txCtx, `
		DELETE FROM plugins p
		USING semrel_dup_map m
		WHERE p.id = m.src_id`)
	if err != nil {
		return 0, 0, fmt.Errorf("delete duplicate plugins: %w", err)
	}

	normalizedTag, err := tx.Exec(txCtx, `
		UPDATE plugins p
		SET    namespace = '@semrel',
		       name      = regexp_replace(p.name, '^(analyzer|condition|generator|hook|provider|updater)-', ''),
		       updated_at = NOW()
		WHERE  (p.namespace IS NULL OR p.namespace = '')
		  AND  p.deleted_at IS NULL
		  AND  p.repository LIKE 'https://github.com/SemRels/%'
		  AND  p.name ~ '^(analyzer|condition|generator|hook|provider|updater)-.+'
		  AND  NOT EXISTS (
				 SELECT 1
				 FROM plugins t
				 WHERE t.namespace = '@semrel'
				   AND t.deleted_at IS NULL
				   AND t.name = regexp_replace(p.name, '^(analyzer|condition|generator|hook|provider|updater)-', '')
		  )`)
	if err != nil {
		return 0, 0, fmt.Errorf("normalize semrel names: %w", err)
	}

	if err = tx.Commit(txCtx); err != nil {
		return 0, 0, fmt.Errorf("commit cleanup tx: %w", err)
	}

	return deletedTag.RowsAffected(), normalizedTag.RowsAffected(), nil
}
