// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The SemRels Authors

// Command dedup removes bare-name (unscoped) plugin rows from the semrel-registry
// database wherever a namespaced counterpart already exists.
//
// This is a one-shot maintenance tool for cleaning up duplicate entries that
// were created before GITHUB_ORG_NAMESPACE was configured in the sync handler.
//
// Usage:
//
//go -C api run cmd/dedup/main.go [-namespace @semrel] [-dry-run]
package main

import (
"context"
"flag"
"fmt"
"log"
"os"

"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
dsn := flag.String("db", "", "PostgreSQL DSN (or set DATABASE_URL env)")
namespace := flag.String("namespace", "@semrel", "Namespace whose bare-name counterparts should be removed")
dryRun := flag.Bool("dry-run", false, "Print what would be deleted without making changes")
flag.Parse()

if *dsn == "" {
*dsn = os.Getenv("DATABASE_URL")
}
if *dsn == "" {
log.Fatal("provide -db <DSN> or set DATABASE_URL")
}

ctx := context.Background()
pool, err := pgxpool.New(ctx, *dsn)
if err != nil {
log.Fatalf("connect db: %v", err)
}
defer pool.Close()

// Find all bare-name plugins that have a namespaced counterpart.
rows, err := pool.Query(ctx, `
SELECT p.id, p.name
FROM   plugins p
WHERE  (p.namespace IS NULL OR p.namespace = '')
  AND  p.deleted_at IS NULL
  AND  EXISTS (
           SELECT 1
           FROM   plugins ns
           WHERE  ns.namespace = $1
             AND  ns.name      = p.name
             AND  ns.deleted_at IS NULL
       )
ORDER BY p.name`, *namespace)
if err != nil {
log.Fatalf("query duplicates: %v", err)
}

type dupRow struct {
id   int64
name string
}
var dupes []dupRow
for rows.Next() {
var r dupRow
if err := rows.Scan(&r.id, &r.name); err != nil {
log.Fatalf("scan row: %v", err)
}
dupes = append(dupes, r)
}
rows.Close()
if err := rows.Err(); err != nil {
log.Fatalf("iterate rows: %v", err)
}

if len(dupes) == 0 {
fmt.Println("No bare-name duplicates found. Nothing to do.")
return
}

fmt.Printf("Found %d bare-name plugin(s) with a %s counterpart:\n", len(dupes), *namespace)
for _, d := range dupes {
fmt.Printf("  id=%-6d  name=%s\n", d.id, d.name)
}

if *dryRun {
fmt.Println("\nDry-run mode — no changes made.")
return
}

// Hard-delete the bare-name rows. The unique indexes do not honour
// deleted_at, so a soft-delete would still block future inserts.
ids := make([]int64, len(dupes))
for i, d := range dupes {
ids[i] = d.id
}

tag, err := pool.Exec(ctx, `DELETE FROM plugins WHERE id = ANY($1)`, ids)
if err != nil {
log.Fatalf("delete duplicates: %v", err)
}

fmt.Printf("\nDeleted %d bare-name plugin row(s).\n", tag.RowsAffected())
}
