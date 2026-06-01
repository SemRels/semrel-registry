// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The SemRels Authors

// Command seed imports plugins.json into the semrel-registry PostgreSQL database.
// Usage: go run scripts/seed/main.go -db <DSN> -file <plugins.json>
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PluginVersion struct {
	Version     string            `json:"version"`
	ReleaseDate string            `json:"releaseDate"`
	DownloadURL string            `json:"downloadUrl"`
	Changelog   string            `json:"changelog"`
	Prerelease  bool              `json:"prerelease"`
	Checksums   map[string]string `json:"checksums"`
	Compatibility struct {
		MinSemrelVersion string `json:"minSemrelVersion"`
		GRPCVersion      string `json:"gRPCVersion"`
	} `json:"compatibility"`
}

type Plugin struct {
	Namespace   string          `json:"namespace"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	Homepage    string          `json:"homepage"`
	Repository  string          `json:"repository"`
	License     string          `json:"license"`
	Category    string          `json:"category"`
	Tags        []string        `json:"tags"`
	Versions    []PluginVersion `json:"versions"`
}

type Registry struct {
	SchemaVersion int      `json:"schemaVersion"`
	GeneratedAt   string   `json:"generatedAt"`
	Plugins       []Plugin `json:"plugins"`
}

func main() {
	dsn := flag.String("db", "postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable", "PostgreSQL DSN")
	file := flag.String("file", "plugins.json", "Path to plugins.json")
	flag.Parse()

	data, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read %s: %v", *file, err)
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		log.Fatalf("parse plugins.json: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	log.Printf("Seeding %d plugins from %s...", len(registry.Plugins), *file)

	inserted, updated, skipped := 0, 0, 0

	for _, p := range registry.Plugins {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}

		var pluginID int64
		var err error

		// Use the correct partial-index conflict target depending on whether
		// the plugin has a namespace or not.
		if p.Namespace != "" {
			err = pool.QueryRow(ctx, `
				INSERT INTO plugins (namespace, name, description, author, category, repository, license, tags, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
				ON CONFLICT (namespace, name) WHERE namespace IS NOT NULL DO UPDATE SET
					description = EXCLUDED.description,
					author      = EXCLUDED.author,
					category    = EXCLUDED.category,
					repository  = EXCLUDED.repository,
					license     = EXCLUDED.license,
					tags        = EXCLUDED.tags,
					updated_at  = NOW()
				RETURNING id`,
				p.Namespace, p.Name, p.Description, p.Author, p.Category, p.Repository, p.License, tags,
			).Scan(&pluginID)
		} else {
			err = pool.QueryRow(ctx, `
				INSERT INTO plugins (name, description, author, category, repository, license, tags, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
				ON CONFLICT (name) WHERE namespace IS NULL DO UPDATE SET
					description = EXCLUDED.description,
					author      = EXCLUDED.author,
					category    = EXCLUDED.category,
					repository  = EXCLUDED.repository,
					license     = EXCLUDED.license,
					tags        = EXCLUDED.tags,
					updated_at  = NOW()
				RETURNING id`,
				p.Name, p.Description, p.Author, p.Category, p.Repository, p.License, tags,
			).Scan(&pluginID)
		}
		if err != nil {
			log.Printf("  ✗ %s: upsert failed: %v", p.Name, err)
			skipped++
			continue
		}

		if len(p.Versions) == 0 {
			log.Printf("  ↷ %s (no versions)", p.Name)
			inserted++
			continue
		}

		versionsAdded := 0
		for _, v := range p.Versions {
			var releaseDate *time.Time
			if v.ReleaseDate != "" {
				t, err := time.Parse(time.RFC3339, v.ReleaseDate)
				if err == nil {
					releaseDate = &t
				}
			}

			var versionID int64
			err := pool.QueryRow(ctx, `
				INSERT INTO plugin_versions (plugin_id, version, release_date, changelog, download_url, prerelease, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW())
				ON CONFLICT (plugin_id, version) DO UPDATE SET
					release_date = EXCLUDED.release_date,
					changelog    = EXCLUDED.changelog,
					download_url = EXCLUDED.download_url,
					prerelease   = EXCLUDED.prerelease
				RETURNING id`,
				pluginID, v.Version, releaseDate, v.Changelog, v.DownloadURL, v.Prerelease,
			).Scan(&versionID)
			if err != nil {
				log.Printf("    ✗ version %s: %v", v.Version, err)
				continue
			}

			for platform, hash := range v.Checksums {
				_, err := pool.Exec(ctx, `
					INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
					VALUES ($1, $2, 'sha256', $3)
					ON CONFLICT DO NOTHING`,
					versionID, platform, hash,
				)
				if err != nil {
					log.Printf("    ✗ checksum %s/%s: %v", v.Version, platform, err)
				}
			}

			versionsAdded++
		}

		log.Printf("  ✓ %s (%d versions)", p.Name, versionsAdded)
		inserted++
	}

	fmt.Printf("\nSeed complete: %d upserted, %d skipped\n", inserted+updated, skipped)
}
