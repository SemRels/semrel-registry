package models

import "time"

type Plugin struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Author        string          `json:"author"`
	Category      string          `json:"category"`
	Repository    string          `json:"repository"`
	License       string          `json:"license"`
	Tags          []string        `json:"tags,omitempty"`
	Versions      []PluginVersion `json:"versions,omitempty"`
	LatestVersion string          `json:"latestVersion,omitempty"`
	Downloads     int64           `json:"downloads"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	DeletedAt     *time.Time      `json:"deletedAt,omitempty"`
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
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Author      *string   `json:"author"`
	Category    *string   `json:"category"`
	Repository  *string   `json:"repository"`
	License     *string   `json:"license"`
	Tags        *[]string `json:"tags"`
}

func (p PluginPatch) Empty() bool {
	return p.Name == nil &&
		p.Description == nil &&
		p.Author == nil &&
		p.Category == nil &&
		p.Repository == nil &&
		p.License == nil &&
		p.Tags == nil
}
