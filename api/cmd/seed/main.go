// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The SemRels Authors

// Command seed imports plugins.json into the semrel-registry PostgreSQL database.
// Usage: go run scripts/seed/main.go -db <DSN> -file <plugins.json>
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/SemRels/semrel-registry/api/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := flag.String("db", "postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable", "PostgreSQL DSN")
	file := flag.String("file", "plugins.json", "Path to plugins.json")
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	result, err := database.ImportSeedFile(ctx, pool, *file)
	if err != nil {
		log.Fatalf("seed %s: %v", *file, err)
	}
	fmt.Printf("Seed complete: %d plugins and %d versions upserted\n",
		result.Plugins, result.Versions)
}
