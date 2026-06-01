package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
)

type seedPlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Category    string   `json:"category"`
	Repository  string   `json:"repository"`
	License     string   `json:"license"`
	Tags        []string `json:"tags"`
}

// seedPluginsIfEmpty reads plugins.json and upserts all plugins into the DB
// only when the database contains no plugins yet. Safe to call on every startup.
func seedPluginsIfEmpty(ctx context.Context, svc service.PluginManager, filePath string) error {
	if filePath == "" {
		filePath = "plugins.json"
	}

	// Check if DB already has plugins — skip seeding if so.
	result, err := svc.ListPlugins(ctx, service.ListPluginsParams{Page: 1, Limit: 1})
	if err != nil {
		return err
	}
	if result.Pagination.Total > 0 {
		return nil // already seeded
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file → skip silently
		}
		return err
	}

	var payload struct {
		Plugins []seedPlugin `json:"plugins"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	created, skipped := 0, 0
	for _, sp := range payload.Plugins {
		existing, err := svc.GetPlugin(ctx, sp.Name)
		if err != nil && !errors.Is(err, appErrors.ErrPluginNotFound) {
			skipped++
			continue
		}
		if existing.ID != 0 {
			skipped++
			continue
		}
		_, err = svc.CreatePlugin(ctx, models.Plugin{
			Name:        sp.Name,
			Description: sp.Description,
			Author:      sp.Author,
			Category:    sp.Category,
			Repository:  sp.Repository,
			License:     sp.License,
			Tags:        sp.Tags,
		})
		if err != nil {
			log.Printf("seed: failed to create plugin %q: %v", sp.Name, err)
			skipped++
			continue
		}
		created++
	}

	log.Printf("seed: imported %d plugins from %s (%d skipped)", created, filePath, skipped)
	return nil
}
