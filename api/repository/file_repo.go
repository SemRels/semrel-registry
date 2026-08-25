package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SemRels/semrel-registry/api/database"
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
		if p.DeletedAt == nil && p.Namespace == "" && strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	for i := range all {
		p := &all[i]
		if p.DeletedAt == nil && containsFold(p.Aliases, name) {
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
	ref := namespace + "/" + name
	for i := range all {
		p := &all[i]
		if p.DeletedAt == nil && containsFold(p.Aliases, ref) {
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
	versions := make([]models.PluginVersion, 0, len(p.Versions))
	for _, version := range p.Versions {
		if version.DeletedAt != nil {
			continue
		}
		versions = append(versions, version)
	}
	return versions, nil
}

func (s *fileStore) Create(_ context.Context, plugin *models.Plugin) (int64, error) {
	if plugin == nil {
		return 0, fmt.Errorf("plugin is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAll()
	if err != nil {
		return 0, err
	}
	plugin.Aliases = normalizeAliases(plugin.Aliases)
	if err := ensureUniqueIdentity(all, *plugin, 0); err != nil {
		return 0, fmt.Errorf("create plugin: %w", err)
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
	all, err := s.loadAll()
	if err != nil {
		return err
	}
	plugin.Aliases = normalizeAliases(plugin.Aliases)
	if err := ensureUniqueIdentity(all, *plugin, plugin.ID); err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}

	now := time.Now().UTC()
	existing.Namespace = plugin.Namespace
	existing.Name = plugin.Name
	existing.Aliases = plugin.Aliases
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

func (s *fileStore) Delete(_ context.Context, spec models.PluginDeletionSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(spec.PluginID)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}
	now := time.Now().UTC()
	p.DeletedAt = &now
	p.DeletedBy = spec.DeletedBy
	p.DeletionReason = spec.Reason
	if spec.AnonymizeAuthor {
		p.Author = ""
	}
	if spec.CascadeVersions {
		for i := range p.Versions {
			if p.Versions[i].DeletedAt != nil {
				continue
			}
			p.Versions[i].DeletedAt = &now
			p.Versions[i].DeletedBy = spec.DeletedBy
			p.Versions[i].DeletionReason = spec.Reason
		}
	}
	p.UpdatedAt = now
	return s.savePlugin(p)
}

func (s *fileStore) DeleteVersion(_ context.Context, spec models.VersionDeletionSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(spec.PluginID)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}

	found := false
	for i := range p.Versions {
		if p.Versions[i].ID != spec.VersionID {
			continue
		}
		if p.Versions[i].DeletedAt != nil {
			return appErrors.ErrPluginNotFound
		}
		found = true
		now := time.Now().UTC()
		p.Versions[i].DeletedAt = &now
		p.Versions[i].DeletedBy = spec.DeletedBy
		p.Versions[i].DeletionReason = spec.Reason
		p.UpdatedAt = now
		break
	}
	if !found {
		return appErrors.ErrPluginNotFound
	}
	return s.savePlugin(p)
}

func (s *fileStore) RecordAccountDeletion(_ context.Context, audit models.AccountDeletionAudit) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.loadAccountDeletionAudits()
	if err != nil {
		return err
	}
	events = append(events, audit)
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal account deletions: %w", err)
	}
	return writeFileAtomic(s.accountDeletionLogPath(), data)
}

func (s *fileStore) loadAccountDeletionAudits() ([]models.AccountDeletionAudit, error) {
	data, err := os.ReadFile(s.accountDeletionLogPath())
	if os.IsNotExist(err) {
		return []models.AccountDeletionAudit{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read account deletions: %w", err)
	}
	var events []models.AccountDeletionAudit
	if len(data) == 0 {
		return []models.AccountDeletionAudit{}, nil
	}
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse account deletions: %w", err)
	}
	return events, nil
}

func (s *fileStore) accountDeletionLogPath() string {
	return filepath.Join(s.dataDir, "account_deletions.json")
}

func (s *fileStore) IncrCounters(_ context.Context, pluginID, versionID, views, downloads int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.loadPlugin(pluginID)
	if err != nil {
		return err
	}
	if p.DeletedAt != nil {
		return appErrors.ErrPluginNotFound
	}

	p.Views += views
	p.Downloads += downloads
	p.UpdatedAt = time.Now().UTC()

	if versionID > 0 {
		for i := range p.Versions {
			if p.Versions[i].ID == versionID {
				p.Versions[i].Views += views
				p.Versions[i].Downloads += downloads
				break
			}
		}
	}

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

// ImportSeedRegistry upserts a bundled catalog into a file repository. Existing
// plugins not present in the catalog, including community plugins, are untouched.
func ImportSeedRegistry(ctx context.Context, repo PluginRepository, registry database.SeedRegistry) (database.SeedImportResult, error) {
	seeder, ok := repo.(interface {
		importSeedRegistry(context.Context, database.SeedRegistry) (database.SeedImportResult, error)
	})
	if !ok {
		return database.SeedImportResult{}, fmt.Errorf("repository does not support catalog seeding")
	}
	return seeder.importSeedRegistry(ctx, registry)
}

func (s *fileStore) importSeedRegistry(ctx context.Context, registry database.SeedRegistry) (database.SeedImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAll()
	if err != nil {
		return database.SeedImportResult{}, err
	}
	meta, err := s.loadMeta()
	if err != nil {
		return database.SeedImportResult{}, err
	}
	for _, plugin := range all {
		if plugin.ID >= meta.NextPluginID {
			meta.NextPluginID = plugin.ID + 1
		}
		for _, version := range plugin.Versions {
			if version.ID >= meta.NextVersionID {
				meta.NextVersionID = version.ID + 1
			}
		}
	}

	result := database.SeedImportResult{}
	for _, plugin := range registry.Plugins {
		if err := ctx.Err(); err != nil {
			return database.SeedImportResult{}, err
		}
		if err := applySeedPlugin(&all, meta, plugin); err != nil {
			return database.SeedImportResult{}, fmt.Errorf("import plugin %q: %w", plugin.Name, err)
		}
		result.Plugins++
		result.Versions += len(plugin.Versions)
	}
	if err := s.saveSeedSnapshot(all, meta); err != nil {
		return database.SeedImportResult{}, err
	}
	return result, nil
}

func applySeedPlugin(all *[]models.Plugin, meta *fileMeta, seed database.SeedPlugin) error {
	existingIndex := -1
	for i := range *all {
		if strings.EqualFold((*all)[i].Namespace, seed.Namespace) &&
			strings.EqualFold((*all)[i].Name, seed.Name) {
			existingIndex = i
			break
		}
	}

	now := time.Now().UTC()
	var candidate models.Plugin
	if existingIndex < 0 {
		candidate = models.Plugin{
			ID:        meta.NextPluginID,
			Status:    models.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		meta.NextPluginID++
	} else {
		candidate = (*all)[existingIndex]
	}

	candidate.Namespace = seed.Namespace
	candidate.Name = seed.Name
	candidate.Aliases = normalizeAliases(seed.Aliases)
	candidate.Description = seed.Description
	candidate.Author = seed.Author
	candidate.Category = seed.Category
	candidate.Repository = seed.Repository
	candidate.License = seed.License
	candidate.Tags = append([]string(nil), seed.Tags...)
	if candidate.Status == "" {
		candidate.Status = models.StatusActive
	}
	if err := ensureUniqueIdentity(*all, candidate, candidate.ID); err != nil {
		return fmt.Errorf("upsert metadata: %w", err)
	}

	versions := append([]models.PluginVersion(nil), candidate.Versions...)
	for _, seededVersion := range seed.Versions {
		var releaseDate *time.Time
		if seededVersion.ReleaseDate != "" {
			parsed, parseErr := time.Parse(time.RFC3339, seededVersion.ReleaseDate)
			if parseErr != nil {
				return fmt.Errorf("parse release date for version %q: %w", seededVersion.Version, parseErr)
			}
			releaseDate = &parsed
		}

		index := -1
		for i := range versions {
			if versions[i].Version == seededVersion.Version {
				index = i
				break
			}
		}
		if index < 0 {
			versions = append(versions, models.PluginVersion{
				ID:        meta.NextVersionID,
				PluginID:  candidate.ID,
				Version:   seededVersion.Version,
				CreatedAt: now,
			})
			meta.NextVersionID++
			index = len(versions) - 1
		}
		version := &versions[index]
		version.PluginID = candidate.ID
		version.ReleaseDate = releaseDate
		version.Changelog = seededVersion.Changelog
		version.DownloadURL = seededVersion.DownloadURL
		version.Prerelease = seededVersion.Prerelease
		version.Checksums = cloneChecksums(seededVersion.Checksums)
	}
	sortPluginVersions(versions)
	candidate.Versions = versions
	candidate.LatestVersion = latestStableVersion(versions)

	if existingIndex >= 0 && !reflect.DeepEqual(candidate, (*all)[existingIndex]) {
		candidate.UpdatedAt = now
	}
	if existingIndex < 0 {
		*all = append(*all, candidate)
	} else {
		(*all)[existingIndex] = candidate
	}
	return nil
}

func (s *fileStore) saveSeedSnapshot(plugins []models.Plugin, meta *fileMeta) error {
	stageDir, err := os.MkdirTemp(s.dataDir, ".plugins-seed-")
	if err != nil {
		return fmt.Errorf("create seed staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	for i := range plugins {
		data, marshalErr := json.MarshalIndent(&plugins[i], "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshal plugin %d: %w", plugins[i].ID, marshalErr)
		}
		if writeErr := os.WriteFile(
			filepath.Join(stageDir, fmt.Sprintf("%d.json", plugins[i].ID)),
			data, 0o644,
		); writeErr != nil {
			return fmt.Errorf("stage plugin %d: %w", plugins[i].ID, writeErr)
		}
	}

	backupDir, err := os.MkdirTemp(s.dataDir, ".plugins-backup-")
	if err != nil {
		return fmt.Errorf("reserve seed backup directory: %w", err)
	}
	if err := os.Remove(backupDir); err != nil {
		return fmt.Errorf("prepare seed backup directory: %w", err)
	}
	defer os.RemoveAll(backupDir)

	if err := os.Rename(s.pluginsDir(), backupDir); err != nil {
		return fmt.Errorf("backup plugin catalog: %w", err)
	}
	if err := os.Rename(stageDir, s.pluginsDir()); err != nil {
		if restoreErr := os.Rename(backupDir, s.pluginsDir()); restoreErr != nil {
			return fmt.Errorf("install seeded catalog: %w (restore failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("install seeded catalog: %w", err)
	}

	if err := s.saveMeta(meta); err != nil {
		failedDir := stageDir
		moveErr := os.Rename(s.pluginsDir(), failedDir)
		restoreErr := os.Rename(backupDir, s.pluginsDir())
		if moveErr != nil || restoreErr != nil {
			return fmt.Errorf("save seeded metadata: %w (rollback failed: move=%v restore=%v)",
				err, moveErr, restoreErr)
		}
		return fmt.Errorf("save seeded metadata: %w", err)
	}
	return nil
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
				haystack := strings.ToLower(p.Name + " " + strings.Join(p.Aliases, " ") + " " + p.Description + " " + p.Author + " " + p.Repository)
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

func containsFold(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func normalizeAliases(aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		key := strings.ToLower(alias)
		if alias == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, alias)
	}
	sort.Strings(result)
	return result
}

func cloneChecksums(checksums map[string]string) map[string]string {
	if checksums == nil {
		return nil
	}
	result := make(map[string]string, len(checksums))
	for platform, checksum := range checksums {
		result[platform] = checksum
	}
	return result
}

func sortPluginVersions(versions []models.PluginVersion) {
	sort.SliceStable(versions, func(i, j int) bool {
		left, right := versions[i], versions[j]
		if left.ReleaseDate != nil && right.ReleaseDate != nil && !left.ReleaseDate.Equal(*right.ReleaseDate) {
			return left.ReleaseDate.After(*right.ReleaseDate)
		}
		if left.ReleaseDate != nil {
			return true
		}
		if right.ReleaseDate != nil {
			return false
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.After(right.CreatedAt)
		}
		return left.Version > right.Version
	})
}

func latestStableVersion(versions []models.PluginVersion) string {
	for _, version := range versions {
		if !version.Prerelease {
			return version.Version
		}
	}
	return ""
}

func ensureUniqueIdentity(all []models.Plugin, candidate models.Plugin, excludeID int64) error {
	refs := make(map[string]struct{}, len(candidate.Aliases)+1)
	refs[strings.ToLower(candidate.Ref())] = struct{}{}
	for _, alias := range candidate.Aliases {
		key := strings.ToLower(strings.TrimSpace(alias))
		if _, exists := refs[key]; exists {
			return appErrors.ErrDuplicatePlugin
		}
		refs[key] = struct{}{}
	}
	for _, plugin := range all {
		if plugin.DeletedAt != nil || plugin.ID == excludeID {
			continue
		}
		if _, exists := refs[strings.ToLower(plugin.Ref())]; exists {
			return appErrors.ErrDuplicatePlugin
		}
		for _, alias := range plugin.Aliases {
			if _, exists := refs[strings.ToLower(strings.TrimSpace(alias))]; exists {
				return appErrors.ErrDuplicatePlugin
			}
		}
	}
	return nil
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
