package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
)

// fileStore is a file-backed implementation of PluginRepository.
// Each plugin (including its versions) is stored as a JSON file under
// {dataDir}/plugins/{id}.json. A meta.json file tracks the next available
// numeric IDs so that IDs remain stable across restarts.
//
// All state mutations are protected by a single RWMutex so the store is safe
// for concurrent use within a single process.  For multi-instance deployments
// mount the data directory on shared storage (NFS, S3 FUSE, etc.).
type fileStore struct {
	mu      sync.RWMutex
	dataDir string
}

// fileMeta holds the autoincrement counters persisted in meta.json.
type fileMeta struct {
	NextPluginID  int64 `json:"next_plugin_id"`
	NextVersionID int64 `json:"next_version_id"`
}

// pluginFile is the on-disk representation – the Plugin struct with versions
// embedded (checksums are part of each PluginVersion already).
type pluginFile struct {
	models.Plugin
	Versions []versionFile `json:"versions"`
}

type versionFile = models.PluginVersion

// NewFileRepository returns a PluginRepository that persists data as JSON files
// inside dataDir.  The directory (and its sub-directories) are created on first
// use if they do not exist yet.
func NewFileRepository(dataDir string) (PluginRepository, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("file repository: dataDir must not be empty")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "plugins"), 0o755); err != nil {
		return nil, fmt.Errorf("file repository: create data directory: %w", err)
	}
	return &fileStore{dataDir: dataDir}, nil
}

// -------------------------------------------------------------------------
// PluginRepository interface
// -------------------------------------------------------------------------

func (s *fileStore) GetAll(_ context.Context, limit, offset int, filters ...Filter) ([]models.Plugin, error) {
	s.mu.RLock()
	all, err := s.loadAll()
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}

	// Apply filters in memory.
	result := make([]models.Plugin, 0, len(all))
	for _, p := range all {
		if p.DeletedAt != nil {
			continue
		}
		if !matchesFilters(p, filters) {
			continue
		}
		result = append(result, p)
	}

	// Sort – respect the first SortFilter, default to name ASC.
	sortField, sortDesc := "name", false
	for _, f := range filters {
		if sf, ok := f.(SortFilter); ok {
			if nf := normalizeSortField(sf.Field); nf != "" {
				sortField = nf
			}
			sortDesc = strings.ToUpper(strings.TrimSpace(sf.Direction)) == "DESC"
			break
		}
	}
	sortPlugins(result, sortField, sortDesc)

	// Pagination.
	if offset > 0 {
		if offset >= len(result) {
			return []models.Plugin{}, nil
		}
		result = result[offset:]
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

func (s *fileStore) GetByID(_ context.Context, id int64) (*models.Plugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.loadPlugin(id)
	if err != nil {
		return nil, err
	}
	if p.DeletedAt != nil {
		return nil, appErrors.ErrPluginNotFound
	}
	return p, nil
}

func (s *fileStore) GetByName(_ context.Context, name string) (*models.Plugin, error) {
	s.mu.RLock()
	all, err := s.loadAll()
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	for i := range all {
		p := &all[i]
		if p.DeletedAt == nil && strings.EqualFold(p.Name, name) && p.Namespace == "" {
			return p, nil
		}
	}
	return nil, appErrors.ErrPluginNotFound
}

func (s *fileStore) GetByNamespacedName(_ context.Context, namespace, name string) (*models.Plugin, error) {
	s.mu.RLock()
	all, err := s.loadAll()
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	for i := range all {
		p := &all[i]
		if p.DeletedAt == nil &&
			strings.EqualFold(p.Namespace, namespace) &&
			strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return nil, appErrors.ErrPluginNotFound
}

func (s *fileStore) GetVersions(_ context.Context, pluginID int64) ([]models.PluginVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.loadPlugin(pluginID)
	if err != nil {
		return nil, err
	}
	versions := p.Versions
	if versions == nil {
		versions = []models.PluginVersion{}
	}
	return versions, nil
}

func (s *fileStore) Create(_ context.Context, plugin *models.Plugin) (int64, error) {
	if plugin == nil {
		return 0, fmt.Errorf("plugin is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Duplicate check.
	all, err := s.loadAll()
	if err != nil {
		return 0, err
	}
	for _, p := range all {
		if p.DeletedAt != nil {
			continue
		}
		if strings.EqualFold(p.Namespace, plugin.Namespace) &&
			strings.EqualFold(p.Name, plugin.Name) {
			return 0, fmt.Errorf("create plugin: %w", appErrors.ErrDuplicatePlugin)
		}
	}

	meta, err := s.loadMeta()
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	plugin.ID = meta.NextPluginID
	plugin.CreatedAt = now
	plugin.UpdatedAt = now
	if plugin.Versions == nil {
		plugin.Versions = []models.PluginVersion{}
	}

	meta.NextPluginID++
	if err := s.saveMeta(meta); err != nil {
		return 0, err
	}
	if err := s.savePlugin(plugin); err != nil {
		return 0, err
	}
	return plugin.ID, nil
}

func (s *fileStore) Update(_ context.Context, plugin *models.Plugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.loadPlugin(plugin.ID)
	if err != nil {
		return err
	}
	if existing.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}

	now := time.Now().UTC()
	existing.Namespace = plugin.Namespace
	existing.Name = plugin.Name
	existing.Description = plugin.Description
	existing.Author = plugin.Author
	existing.Category = plugin.Category
	existing.Repository = plugin.Repository
	existing.License = plugin.License
	existing.Tags = plugin.Tags
	existing.UpdatedAt = now

	plugin.UpdatedAt = now
	return s.savePlugin(existing)
}

func (s *fileStore) UpdateStatus(_ context.Context, id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(id)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
	return s.savePlugin(p)
}

func (s *fileStore) UpdateValidationChecks(_ context.Context, id int64, checksJSON []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(id)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}
	now := time.Now().UTC()
	p.ValidationChecks = checksJSON
	p.ValidatedAt = &now
	p.UpdatedAt = now
	return s.savePlugin(p)
}

func (s *fileStore) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(id)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}
	now := time.Now().UTC()
	p.DeletedAt = &now
	p.UpdatedAt = now
	return s.savePlugin(p)
}

func (s *fileStore) AddVersion(_ context.Context, version *models.PluginVersion) (int64, error) {
	if version == nil {
		return 0, fmt.Errorf("plugin version is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(version.PluginID)
	if err != nil {
		return 0, err
	}
	if p.DeletedAt != nil {
		return 0, appErrors.ErrPluginNotFound
	}

	// Duplicate version check.
	for _, v := range p.Versions {
		if v.Version == version.Version {
			return 0, fmt.Errorf("create plugin version: %w", appErrors.ErrDuplicatePlugin)
		}
	}

	meta, err := s.loadMeta()
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	version.ID = meta.NextVersionID
	version.CreatedAt = now
	if version.Checksums == nil {
		version.Checksums = make(map[string]string)
	}

	meta.NextVersionID++
	if err := s.saveMeta(meta); err != nil {
		return 0, err
	}

	p.Versions = append(p.Versions, *version)
	// Keep newest first (matching DB ordering: release_date DESC, created_at DESC).
	sort.Slice(p.Versions, func(i, j int) bool {
		vi, vj := p.Versions[i], p.Versions[j]
		if vi.ReleaseDate != nil && vj.ReleaseDate != nil {
			if !vi.ReleaseDate.Equal(*vj.ReleaseDate) {
				return vi.ReleaseDate.After(*vj.ReleaseDate)
			}
		} else if vi.ReleaseDate != nil {
			return true
		} else if vj.ReleaseDate != nil {
			return false
		}
		return vi.CreatedAt.After(vj.CreatedAt)
	})

	if err := s.savePlugin(p); err != nil {
		return 0, err
	}
	return version.ID, nil
}

// -------------------------------------------------------------------------
// Internal helpers
// -------------------------------------------------------------------------

func (s *fileStore) pluginsDir() string {
	return filepath.Join(s.dataDir, "plugins")
}

func (s *fileStore) pluginPath(id int64) string {
	return filepath.Join(s.pluginsDir(), fmt.Sprintf("%d.json", id))
}

func (s *fileStore) metaPath() string {
	return filepath.Join(s.dataDir, "meta.json")
}

func (s *fileStore) loadMeta() (*fileMeta, error) {
	data, err := os.ReadFile(s.metaPath())
	if os.IsNotExist(err) {
		return &fileMeta{NextPluginID: 1, NextVersionID: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read meta: %w", err)
	}
	var meta fileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse meta: %w", err)
	}
	return &meta, nil
}

func (s *fileStore) saveMeta(meta *fileMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return writeFileAtomic(s.metaPath(), data)
}

func (s *fileStore) loadPlugin(id int64) (*models.Plugin, error) {
	data, err := os.ReadFile(s.pluginPath(id))
	if os.IsNotExist(err) {
		return nil, appErrors.ErrPluginNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin %d: %w", id, err)
	}
	var p models.Plugin
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plugin %d: %w", id, err)
	}
	if p.Versions == nil {
		p.Versions = []models.PluginVersion{}
	}
	return &p, nil
}

func (s *fileStore) savePlugin(p *models.Plugin) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin %d: %w", p.ID, err)
	}
	return writeFileAtomic(s.pluginPath(p.ID), data)
}

// loadAll reads every plugin file from the plugins directory.
func (s *fileStore) loadAll() ([]models.Plugin, error) {
	entries, err := os.ReadDir(s.pluginsDir())
	if err != nil {
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}
	plugins := make([]models.Plugin, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.pluginsDir(), e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read plugin file %s: %w", e.Name(), err)
		}
		var p models.Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse plugin file %s: %w", e.Name(), err)
		}
		if p.Versions == nil {
			p.Versions = []models.PluginVersion{}
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// writeFileAtomic writes data to path via a temp file + rename to avoid
// partial writes being visible to concurrent readers.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// -------------------------------------------------------------------------
// In-memory filter application
// -------------------------------------------------------------------------

func matchesFilters(p models.Plugin, filters []Filter) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		switch ft := f.(type) {
		case CategoryFilter:
			cat := strings.TrimSpace(ft.Category)
			if cat != "" && !strings.EqualFold(p.Category, cat) {
				return false
			}
		case SearchFilter:
			q := strings.ToLower(strings.TrimSpace(ft.Query))
			if q != "" {
				haystack := strings.ToLower(p.Name + " " + p.Description + " " + p.Author + " " + p.Repository)
				if !strings.Contains(haystack, q) {
					return false
				}
			}
		case AuthorFilter:
			a := strings.TrimSpace(ft.Author)
			if a != "" && !strings.EqualFold(p.Author, a) {
				return false
			}
		case StatusFilter:
			if len(ft.Statuses) > 0 {
				matched := false
				for _, s := range ft.Statuses {
					if strings.EqualFold(p.Status, s) {
						matched = true
						break
					}
				}
				if !matched {
					return false
				}
			}
		case NamespaceFilter:
			ns := strings.TrimSpace(ft.Namespace)
			if ns != "" && !strings.EqualFold(p.Namespace, ns) {
				return false
			}
		// SortFilter is handled separately in GetAll – skip here.
		}
	}
	return true
}

func sortPlugins(plugins []models.Plugin, field string, desc bool) {
	sort.SliceStable(plugins, func(i, j int) bool {
		var less bool
		switch field {
		case "category":
			less = plugins[i].Category < plugins[j].Category
		case "created_at":
			less = plugins[i].CreatedAt.Before(plugins[j].CreatedAt)
		case "updated_at":
			less = plugins[i].UpdatedAt.Before(plugins[j].UpdatedAt)
		default: // "name"
			less = strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
		}
		if desc {
			return !less
		}
		return less
	})
}
