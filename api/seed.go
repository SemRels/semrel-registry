package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/SemRels/semrel-registry/api/database"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

const packagedCatalogPath = "/app/plugins.json"

type catalogSource struct {
	path     string
	required bool
}

func resolveCatalogSource(configuredPath, environment string) catalogSource {
	if path := strings.TrimSpace(configuredPath); path != "" {
		return catalogSource{path: path, required: true}
	}
	if strings.EqualFold(environment, "prod") || strings.EqualFold(environment, "production") {
		return catalogSource{path: packagedCatalogPath, required: true}
	}
	return catalogSource{path: "plugins.json", required: false}
}

// seedStartupCatalog imports the complete catalog for either storage backend.
func seedStartupCatalog(ctx context.Context, repo repository.PluginRepository, pool *pgxpool.Pool, source catalogSource) error {
	var (
		result database.SeedImportResult
		err    error
	)
	if pool != nil {
		result, err = database.ImportSeedFile(ctx, pool, source.path)
	} else {
		var registry database.SeedRegistry
		registry, err = database.ReadSeedFile(source.path)
		if err == nil {
			result, err = repository.ImportSeedRegistry(ctx, repo, registry)
		}
	}
	if errors.Is(err, os.ErrNotExist) && !source.required {
		log.Printf("seed: optional development catalog %s not found; skipping", source.path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("seed bundled catalog %q: %w", source.path, err)
	}
	log.Printf("seed: upserted %d plugins and %d versions from %s",
		result.Plugins, result.Versions, source.path)
	return nil
}

// seedPlugins retains the PostgreSQL importer entry point used by integration
// tests and maintenance code. Its per-plugin transaction semantics are unchanged.
func seedPlugins(ctx context.Context, pool *pgxpool.Pool, filePath string) error {
	if filePath == "" {
		filePath = "plugins.json"
	}
	return seedStartupCatalog(ctx, nil, pool, catalogSource{path: filePath, required: true})
}
