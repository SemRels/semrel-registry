// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The SemRels Authors

// Command dedup merges duplicate first-party rows and canonicalizes surviving
// SemRels plugin identities.
//
// Usage:
//
//go -C api run cmd/dedup/main.go [-dry-run]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/SemRels/semrel-registry/api/database"
)

func main() {
	dsn := flag.String("db", "", "PostgreSQL DSN (or set DATABASE_URL env)")
	dryRun := flag.Bool("dry-run", false, "Print what would be merged or normalized without making changes")
	flag.Parse()

	if *dsn == "" {
		*dsn = os.Getenv("DATABASE_URL")
	}
	if *dsn == "" {
		log.Fatal("provide -db <DSN> or set DATABASE_URL")
	}

	db, err := database.Connect(*dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if *dryRun {
		rows, err := db.Pool().Query(ctx, `
			SELECT p.id, COALESCE(p.namespace, ''), p.name,
			       regexp_replace(p.repository, '^.*/', '') AS canonical_name,
			       EXISTS (
			           SELECT 1 FROM plugins target
			           WHERE target.id <> p.id
			             AND target.namespace = '@semrel'
			             AND target.name = regexp_replace(p.repository, '^.*/', '')
			             AND target.deleted_at IS NULL
			       ) AS duplicate
			FROM plugins p
			WHERE p.deleted_at IS NULL
			  AND p.repository ~* '^https://github\.com/SemRels/(analyzer|condition|generator|hook|packager|provider|publisher|updater)-'
			  AND NOT (COALESCE(p.namespace, '') = '@semrel'
			           AND p.name = regexp_replace(p.repository, '^.*/', ''))
			ORDER BY canonical_name, p.id`)
		if err != nil {
			log.Fatalf("query candidates: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var id int64
			var namespace, name, canonical string
			var duplicate bool
			if err := rows.Scan(&id, &namespace, &name, &canonical, &duplicate); err != nil {
				log.Fatalf("scan candidate: %v", err)
			}
			action := "normalize"
			if duplicate {
				action = "merge"
			}
			fmt.Printf("%-9s id=%-6d %s/%s -> @semrel/%s\n", action, id, namespace, name, canonical)
			count++
		}
		if err := rows.Err(); err != nil {
			log.Fatalf("iterate candidates: %v", err)
		}
		fmt.Printf("Dry run: %d row(s) would change.\n", count)
		return
	}

	deleted, normalized, err := db.CleanupSemrelDuplicates(ctx)
	if err != nil {
		log.Fatalf("canonical cleanup: %v", err)
	}
	fmt.Printf("Merged %d duplicate row(s); normalized %d first-party row(s).\n", deleted, normalized)
}
