package models

import (
	"encoding/json"
	"time"
)

// StatusActive, StatusPending, StatusRejected are the valid plugin statuses.
const (
	StatusActive   = "active"
	StatusPending  = "pending"
	StatusRejected = "rejected"
)

type Plugin struct {
	ID               int64           `json:"id"`
	Namespace        string          `json:"namespace,omitempty"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Author           string          `json:"author"`
	Category         string          `json:"category"`
	Repository       string          `json:"repository"`
	License          string          `json:"license"`
	Status           string          `json:"status"`
	Tags             []string        `json:"tags,omitempty"`
	Versions         []PluginVersion `json:"versions,omitempty"`
	LatestVersion    string          `json:"latestVersion,omitempty"`
	Downloads        int64           `json:"downloads"`
	ValidationChecks json.RawMessage `json:"validationChecks,omitempty"`
	ValidatedAt      *time.Time      `json:"validatedAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	DeletedAt        *time.Time      `json:"deletedAt,omitempty"`
}

type PluginVersion struct {
	ID          int64             `json:"id"`
	PluginID    int64             `json:"pluginId"`
	Version     string            `json:"version"`
	ReleaseDate *time.Time        `json:"releaseDate,omitempty"`
	Changelog   string            `json:"changelog"`
	DownloadURL string            `json:"downloadUrl"`
	Checksums   map[string]string `json:"checksums,omitempty"`
	Prerelease  bool              `json:"prerelease"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type PluginPatch struct {
	Namespace   *string   `json:"namespace"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Author      *string   `json:"author"`
	Category    *string   `json:"category"`
	Repository  *string   `json:"repository"`
	License     *string   `json:"license"`
	Tags        *[]string `json:"tags"`
}

// Ref returns the canonical reference for the plugin: "@namespace/name" if a
// namespace is set, otherwise just "name". Use Ref() whenever converting a Plugin
// back into a lookup key for service calls.
func (p Plugin) Ref() string {
	if p.Namespace != "" {
		return p.Namespace + "/" + p.Name
	}
	return p.Name
}

func (p PluginPatch) Empty() bool {
	return p.Namespace == nil &&
		p.Name == nil &&
		p.Description == nil &&
		p.Author == nil &&
		p.Category == nil &&
		p.Repository == nil &&
		p.License == nil &&
		p.Tags == nil
}
