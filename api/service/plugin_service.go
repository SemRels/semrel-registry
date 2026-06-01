package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
)

const (
	defaultListLimit     = 20
	defaultVersionLimit  = 20
	maxListLimit         = 100
	maxNameLength        = 255
	maxNamespaceLength   = 100
	maxDescriptionLength = 4000
	maxAuthorLength      = 255
	maxCategoryLength    = 50
	maxRepositoryLength  = 2048
	maxLicenseLength     = 50
	maxTagLength         = 100
	maxVersionLength     = 50
	maxChecksumLength    = 255
	maxSearchLength      = 255
	maxChangelogLength   = 20000
)

var pluginNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// namespacePattern matches scoped namespace identifiers such as @semrel or @google.
var namespacePattern = regexp.MustCompile(`^@[a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?$`)

var allowedSorts = map[string]struct{}{
	"":           {},
	"name":       {},
	"category":   {},
	"created_at": {},
	"updated_at": {},
	"downloads":  {},
}

type ListPluginsParams struct {
	Page      int
	Limit     int
	Category  string
	Search    string
	Sort      string
	SortDir   string // "asc" or "desc"; defaults to "asc"
	Namespace string   // when set, filter by namespace (e.g. "@semrel")
	Author    string   // when set, only return plugins by this author (exact, case-insensitive)
	Statuses  []string // when set, filter by status (e.g. ["active"] or ["pending"]); default: ["active"]
}

type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

type PluginListResult struct {
	Data       []models.Plugin `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

type PluginManager interface {
	ListPlugins(ctx context.Context, params ListPluginsParams) (PluginListResult, error)
	GetPlugin(ctx context.Context, ref string) (models.Plugin, error)
	ListVersions(ctx context.Context, ref string, limit, offset int) ([]models.PluginVersion, error)
	CreatePlugin(ctx context.Context, plugin models.Plugin) (models.Plugin, error)
	SubmitPlugin(ctx context.Context, plugin models.Plugin) (models.Plugin, error)
	UpdatePlugin(ctx context.Context, ref string, patch models.PluginPatch) (models.Plugin, error)
	DeletePlugin(ctx context.Context, ref string) error
	CreateVersion(ctx context.Context, ref string, version models.PluginVersion) (models.PluginVersion, error)
	ApprovePlugin(ctx context.Context, ref string) (models.Plugin, error)
	RejectPlugin(ctx context.Context, ref string) (models.Plugin, error)
	UpdateValidationChecks(ctx context.Context, id int64, checksJSON []byte) error
}

type PluginService struct {
	repo repository.PluginRepository
}

func NewPluginService(repo repository.PluginRepository) *PluginService {
	return &PluginService{repo: repo}
}

func (s *PluginService) ListPlugins(ctx context.Context, params ListPluginsParams) (PluginListResult, error) {
	params = normalizeListParams(params)
	if err := validateListParams(params); err != nil {
		return PluginListResult{}, err
	}

	filters := make([]repository.Filter, 0, 6)
	if params.Category != "" {
		filters = append(filters, repository.CategoryFilter{Category: params.Category})
	}
	if params.Search != "" {
		filters = append(filters, repository.SearchFilter{Query: params.Search})
	}
	if params.Namespace != "" {
		filters = append(filters, repository.NamespaceFilter{Namespace: params.Namespace})
	}
	if params.Author != "" {
		filters = append(filters, repository.AuthorFilter{Author: params.Author})
	}
	if len(params.Statuses) > 0 {
		filters = append(filters, repository.StatusFilter{Statuses: params.Statuses})
	}
	if params.Sort != "" {
		dir := "ASC"
		if strings.EqualFold(params.SortDir, "desc") {
			dir = "DESC"
		}
		filters = append(filters, repository.SortFilter{Field: params.Sort, Direction: dir})
	}

	offset := (params.Page - 1) * params.Limit
	plugins, err := s.repo.GetAll(ctx, params.Limit, offset, filters...)
	if err != nil {
		return PluginListResult{}, err
	}

	total, err := s.countPlugins(ctx, params)
	if err != nil {
		return PluginListResult{}, err
	}

	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return PluginListResult{
		Data: plugins,
		Pagination: Pagination{
			Page:  params.Page,
			Limit: params.Limit,
			Total: total,
			Pages: pages,
		},
	}, nil
}

func (s *PluginService) GetPlugin(ctx context.Context, ref string) (models.Plugin, error) {
	if err := validatePluginRef(ref); err != nil {
		return models.Plugin{}, err
	}
	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.Plugin{}, err
	}
	return *plugin, nil
}

func (s *PluginService) ListVersions(ctx context.Context, ref string, limit, offset int) ([]models.PluginVersion, error) {
	if err := validatePluginRef(ref); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = defaultVersionLimit
	}
	if limit < 1 || limit > maxListLimit {
		return nil, &appErrors.ValidationError{Field: "limit", Issue: fmt.Sprintf("must be between 1 and %d", maxListLimit)}
	}
	if offset < 0 {
		return nil, &appErrors.ValidationError{Field: "offset", Issue: "must be greater than or equal to 0"}
	}

	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return nil, err
	}

	versions, err := s.repo.GetVersions(ctx, plugin.ID)
	if err != nil {
		return nil, err
	}
	if offset >= len(versions) {
		return []models.PluginVersion{}, nil
	}
	end := offset + limit
	if end > len(versions) {
		end = len(versions)
	}

	return versions[offset:end], nil
}

func (s *PluginService) CreatePlugin(ctx context.Context, plugin models.Plugin) (models.Plugin, error) {
	plugin = normalizePlugin(plugin)
	if plugin.Status == "" {
		plugin.Status = models.StatusActive
	}
	if err := validatePlugin(plugin, true); err != nil {
		return models.Plugin{}, err
	}

	id, err := s.repo.Create(ctx, &plugin)
	if err != nil {
		return models.Plugin{}, err
	}

	for i := range plugin.Versions {
		plugin.Versions[i].PluginID = id
		if _, err := s.repo.AddVersion(ctx, &plugin.Versions[i]); err != nil {
			return models.Plugin{}, err
		}
	}

	created, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return models.Plugin{}, err
	}
	return *created, nil
}

// SubmitPlugin creates a plugin with status=pending for community review.
func (s *PluginService) SubmitPlugin(ctx context.Context, plugin models.Plugin) (models.Plugin, error) {
	plugin = normalizePlugin(plugin)
	plugin.Status = models.StatusPending
	if err := validatePlugin(plugin, true); err != nil {
		return models.Plugin{}, err
	}

	id, err := s.repo.Create(ctx, &plugin)
	if err != nil {
		return models.Plugin{}, err
	}

	created, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return models.Plugin{}, err
	}
	return *created, nil
}

// ApprovePlugin sets a plugin's status to active.
func (s *PluginService) ApprovePlugin(ctx context.Context, ref string) (models.Plugin, error) {
	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.Plugin{}, err
	}
	if err := s.repo.UpdateStatus(ctx, plugin.ID, models.StatusActive); err != nil {
		return models.Plugin{}, err
	}
	updated, err := s.repo.GetByID(ctx, plugin.ID)
	if err != nil {
		return models.Plugin{}, err
	}
	return *updated, nil
}

// RejectPlugin sets a plugin's status to rejected.
func (s *PluginService) RejectPlugin(ctx context.Context, ref string) (models.Plugin, error) {
	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.Plugin{}, err
	}
	if err := s.repo.UpdateStatus(ctx, plugin.ID, models.StatusRejected); err != nil {
		return models.Plugin{}, err
	}
	updated, err := s.repo.GetByID(ctx, plugin.ID)
	if err != nil {
		return models.Plugin{}, err
	}
	return *updated, nil
}

func (s *PluginService) UpdatePlugin(ctx context.Context, ref string, patch models.PluginPatch) (models.Plugin, error) {
	if err := validatePluginRef(ref); err != nil {
		return models.Plugin{}, err
	}
	patch = normalizePatch(patch)
	if patch.Empty() {
		return models.Plugin{}, &appErrors.ValidationError{Field: "body", Issue: "at least one field is required"}
	}
	if err := validatePatch(patch); err != nil {
		return models.Plugin{}, err
	}

	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.Plugin{}, err
	}
	applyPatch(plugin, patch)

	if err := s.repo.Update(ctx, plugin); err != nil {
		return models.Plugin{}, err
	}

	updated, err := s.repo.GetByID(ctx, plugin.ID)
	if err != nil {
		return models.Plugin{}, err
	}
	return *updated, nil
}

func (s *PluginService) DeletePlugin(ctx context.Context, ref string) error {
	if err := validatePluginRef(ref); err != nil {
		return err
	}
	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, plugin.ID)
}

func (s *PluginService) CreateVersion(ctx context.Context, ref string, version models.PluginVersion) (models.PluginVersion, error) {
	if err := validatePluginRef(ref); err != nil {
		return models.PluginVersion{}, err
	}
	version = normalizeVersion(version)
	if err := validateVersion(version); err != nil {
		return models.PluginVersion{}, err
	}

	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.PluginVersion{}, err
	}
	version.PluginID = plugin.ID

	id, err := s.repo.AddVersion(ctx, &version)
	if err != nil {
		return models.PluginVersion{}, err
	}

	versions, err := s.repo.GetVersions(ctx, plugin.ID)
	if err != nil {
		return models.PluginVersion{}, err
	}
	for _, candidate := range versions {
		if candidate.ID == id {
			return candidate, nil
		}
	}

	return models.PluginVersion{}, appErrors.ErrPluginNotFound
}

func (s *PluginService) countPlugins(ctx context.Context, params ListPluginsParams) (int64, error) {
	plugins, err := s.repo.GetAll(ctx, 0, 0, countFilters(params)...)
	if err != nil {
		return 0, err
	}
	return int64(len(plugins)), nil
}

func countFilters(params ListPluginsParams) []repository.Filter {
	filters := make([]repository.Filter, 0, 5)
	if params.Category != "" {
		filters = append(filters, repository.CategoryFilter{Category: params.Category})
	}
	if params.Search != "" {
		filters = append(filters, repository.SearchFilter{Query: params.Search})
	}
	if params.Namespace != "" {
		filters = append(filters, repository.NamespaceFilter{Namespace: params.Namespace})
	}
	if params.Author != "" {
		filters = append(filters, repository.AuthorFilter{Author: params.Author})
	}
	if len(params.Statuses) > 0 {
		filters = append(filters, repository.StatusFilter{Statuses: params.Statuses})
	}
	return filters
}

func (s *PluginService) lookupPlugin(ctx context.Context, ref string) (*models.Plugin, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return s.repo.GetByID(ctx, id)
	}
	// Support @namespace/name refs such as @semrel/provider-github.
	if strings.HasPrefix(ref, "@") {
		if slash := strings.Index(ref, "/"); slash > 0 {
			ns := ref[:slash]
			name := ref[slash+1:]
			if name != "" {
				return s.repo.GetByNamespacedName(ctx, ns, name)
			}
		}
	}
	return s.repo.GetByName(ctx, ref)
}

func normalizeListParams(params ListPluginsParams) ListPluginsParams {
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Limit == 0 {
		params.Limit = defaultListLimit
	}
	params.Category = strings.TrimSpace(params.Category)
	params.Search = strings.TrimSpace(params.Search)
	params.Sort = strings.TrimSpace(params.Sort)
	params.Namespace = strings.TrimSpace(params.Namespace)
	params.Author = strings.TrimSpace(params.Author)
	return params
}

func validateListParams(params ListPluginsParams) error {
	if params.Page < 1 {
		return &appErrors.ValidationError{Field: "page", Issue: "must be greater than or equal to 1"}
	}
	if params.Limit < 1 || params.Limit > maxListLimit {
		return &appErrors.ValidationError{Field: "limit", Issue: fmt.Sprintf("must be between 1 and %d", maxListLimit)}
	}
	if len(params.Category) > maxCategoryLength {
		return &appErrors.ValidationError{Field: "category", Issue: fmt.Sprintf("must be at most %d characters", maxCategoryLength)}
	}
	if params.Category != "" && !pluginNamePattern.MatchString(params.Category) {
		return &appErrors.ValidationError{Field: "category", Issue: "must contain only letters, numbers, dots, dashes, or underscores"}
	}
	if params.Namespace != "" && !namespacePattern.MatchString(params.Namespace) {
		return &appErrors.ValidationError{Field: "namespace", Issue: "must start with @ followed by lowercase letters, numbers, or hyphens (e.g. @semrel)"}
	}
	if len(params.Search) > maxSearchLength {
		return &appErrors.ValidationError{Field: "search", Issue: fmt.Sprintf("must be at most %d characters", maxSearchLength)}
	}
	if _, ok := allowedSorts[params.Sort]; !ok {
		return &appErrors.ValidationError{Field: "sort", Issue: "must be one of: name, category, created_at, updated_at"}
	}
	return nil
}

func validatePluginRef(ref string) error {
	if strings.TrimSpace(ref) == "" {
		return &appErrors.ValidationError{Field: "id", Issue: "is required"}
	}
	return nil
}

func normalizePlugin(plugin models.Plugin) models.Plugin {
	plugin.Namespace = strings.TrimSpace(plugin.Namespace)
	plugin.Name = strings.TrimSpace(plugin.Name)
	plugin.Description = strings.TrimSpace(plugin.Description)
	plugin.Author = strings.TrimSpace(plugin.Author)
	plugin.Category = strings.TrimSpace(plugin.Category)
	plugin.Repository = strings.TrimSpace(plugin.Repository)
	plugin.License = strings.TrimSpace(plugin.License)
	plugin.Tags = normalizeTags(plugin.Tags)
	for i := range plugin.Versions {
		plugin.Versions[i] = normalizeVersion(plugin.Versions[i])
	}
	return plugin
}

func normalizePatch(patch models.PluginPatch) models.PluginPatch {
	patch.Namespace = trimPointer(patch.Namespace)
	patch.Name = trimPointer(patch.Name)
	patch.Description = trimPointer(patch.Description)
	patch.Author = trimPointer(patch.Author)
	patch.Category = trimPointer(patch.Category)
	patch.Repository = trimPointer(patch.Repository)
	patch.License = trimPointer(patch.License)
	if patch.Tags != nil {
		tags := normalizeTags(*patch.Tags)
		patch.Tags = &tags
	}
	return patch
}

func normalizeVersion(version models.PluginVersion) models.PluginVersion {
	version.Version = strings.TrimSpace(version.Version)
	version.Changelog = strings.TrimSpace(version.Changelog)
	version.DownloadURL = strings.TrimSpace(version.DownloadURL)
	if len(version.Checksums) > 0 {
		normalized := make(map[string]string, len(version.Checksums))
		for key, value := range version.Checksums {
			normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		version.Checksums = normalized
	}
	return version
}

func validatePlugin(plugin models.Plugin, requireName bool) error {
	if plugin.Namespace != "" {
		if len(plugin.Namespace) > maxNamespaceLength {
			return &appErrors.ValidationError{Field: "namespace", Issue: fmt.Sprintf("must be at most %d characters", maxNamespaceLength)}
		}
		if !namespacePattern.MatchString(plugin.Namespace) {
			return &appErrors.ValidationError{Field: "namespace", Issue: "must start with @ followed by lowercase letters, numbers, or hyphens (e.g. @semrel)"}
		}
	}
	if requireName && plugin.Name == "" {
		return &appErrors.ValidationError{Field: "name", Issue: "is required"}
	}
	if plugin.Name != "" {
		if len(plugin.Name) > maxNameLength {
			return &appErrors.ValidationError{Field: "name", Issue: fmt.Sprintf("must be at most %d characters", maxNameLength)}
		}
		if !pluginNamePattern.MatchString(plugin.Name) {
			return &appErrors.ValidationError{Field: "name", Issue: "must contain only letters, numbers, dots, dashes, or underscores"}
		}
	}
	if plugin.Category == "" {
		return &appErrors.ValidationError{Field: "category", Issue: "is required"}
	}
	if len(plugin.Description) > maxDescriptionLength {
		return &appErrors.ValidationError{Field: "description", Issue: fmt.Sprintf("must be at most %d characters", maxDescriptionLength)}
	}
	if len(plugin.Author) > maxAuthorLength {
		return &appErrors.ValidationError{Field: "author", Issue: fmt.Sprintf("must be at most %d characters", maxAuthorLength)}
	}
	if len(plugin.Category) > maxCategoryLength {
		return &appErrors.ValidationError{Field: "category", Issue: fmt.Sprintf("must be at most %d characters", maxCategoryLength)}
	}
	if !pluginNamePattern.MatchString(plugin.Category) {
		return &appErrors.ValidationError{Field: "category", Issue: "must contain only letters, numbers, dots, dashes, or underscores"}
	}
	if len(plugin.Repository) > maxRepositoryLength {
		return &appErrors.ValidationError{Field: "repository", Issue: fmt.Sprintf("must be at most %d characters", maxRepositoryLength)}
	}
	if len(plugin.License) > maxLicenseLength {
		return &appErrors.ValidationError{Field: "license", Issue: fmt.Sprintf("must be at most %d characters", maxLicenseLength)}
	}
	for _, tag := range plugin.Tags {
		if len(tag) > maxTagLength {
			return &appErrors.ValidationError{Field: "tags", Issue: fmt.Sprintf("each tag must be at most %d characters", maxTagLength)}
		}
	}
	for _, version := range plugin.Versions {
		if err := validateVersion(version); err != nil {
			return err
		}
	}
	return nil
}

func validatePatch(patch models.PluginPatch) error {
	if patch.Namespace != nil && *patch.Namespace != "" {
		if len(*patch.Namespace) > maxNamespaceLength {
			return &appErrors.ValidationError{Field: "namespace", Issue: fmt.Sprintf("must be at most %d characters", maxNamespaceLength)}
		}
		if !namespacePattern.MatchString(*patch.Namespace) {
			return &appErrors.ValidationError{Field: "namespace", Issue: "must start with @ followed by lowercase letters, numbers, or hyphens (e.g. @semrel)"}
		}
	}
	if patch.Name != nil {
		if err := validatePlugin(models.Plugin{Name: *patch.Name, Category: "patched"}, true); err != nil {
			if validationErr, ok := err.(*appErrors.ValidationError); ok && validationErr.Field == "category" {
			} else if err != nil {
				return err
			}
		}
	}
	if patch.Description != nil && len(*patch.Description) > maxDescriptionLength {
		return &appErrors.ValidationError{Field: "description", Issue: fmt.Sprintf("must be at most %d characters", maxDescriptionLength)}
	}
	if patch.Author != nil && len(*patch.Author) > maxAuthorLength {
		return &appErrors.ValidationError{Field: "author", Issue: fmt.Sprintf("must be at most %d characters", maxAuthorLength)}
	}
	if patch.Category != nil {
		if *patch.Category == "" {
			return &appErrors.ValidationError{Field: "category", Issue: "is required"}
		}
		if len(*patch.Category) > maxCategoryLength {
			return &appErrors.ValidationError{Field: "category", Issue: fmt.Sprintf("must be at most %d characters", maxCategoryLength)}
		}
		if !pluginNamePattern.MatchString(*patch.Category) {
			return &appErrors.ValidationError{Field: "category", Issue: "must contain only letters, numbers, dots, dashes, or underscores"}
		}
	}
	if patch.Repository != nil && len(*patch.Repository) > maxRepositoryLength {
		return &appErrors.ValidationError{Field: "repository", Issue: fmt.Sprintf("must be at most %d characters", maxRepositoryLength)}
	}
	if patch.License != nil && len(*patch.License) > maxLicenseLength {
		return &appErrors.ValidationError{Field: "license", Issue: fmt.Sprintf("must be at most %d characters", maxLicenseLength)}
	}
	if patch.Tags != nil {
		for _, tag := range *patch.Tags {
			if len(tag) > maxTagLength {
				return &appErrors.ValidationError{Field: "tags", Issue: fmt.Sprintf("each tag must be at most %d characters", maxTagLength)}
			}
		}
	}
	return nil
}

func validateVersion(version models.PluginVersion) error {
	if version.Version == "" {
		return &appErrors.ValidationError{Field: "version", Issue: "is required"}
	}
	if len(version.Version) > maxVersionLength {
		return &appErrors.ValidationError{Field: "version", Issue: fmt.Sprintf("must be at most %d characters", maxVersionLength)}
	}
	if version.DownloadURL == "" {
		return &appErrors.ValidationError{Field: "downloadUrl", Issue: "is required"}
	}
	if len(version.Changelog) > maxChangelogLength {
		return &appErrors.ValidationError{Field: "changelog", Issue: fmt.Sprintf("must be at most %d characters", maxChangelogLength)}
	}
	if len(version.Checksums) == 0 {
		return &appErrors.ValidationError{Field: "checksums", Issue: "at least one checksum is required"}
	}
	for key, value := range version.Checksums {
		if strings.TrimSpace(key) == "" {
			return &appErrors.ValidationError{Field: "checksums", Issue: "platform key cannot be empty"}
		}
		if value == "" {
			return &appErrors.ValidationError{Field: "checksums", Issue: "checksum hash cannot be empty"}
		}
		if len(value) > maxChecksumLength {
			return &appErrors.ValidationError{Field: "checksums", Issue: fmt.Sprintf("checksum hash must be at most %d characters", maxChecksumLength)}
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func enrichPlugin(plugin models.Plugin, includeVersions bool) models.Plugin {
	if plugin.LatestVersion == "" && len(plugin.Versions) > 0 {
		plugin.LatestVersion = plugin.Versions[0].Version
	}
	if !includeVersions {
		plugin.Versions = nil
	}
	return plugin
}

func (s *PluginService) UpdateValidationChecks(ctx context.Context, id int64, checksJSON []byte) error {
	return s.repo.UpdateValidationChecks(ctx, id, checksJSON)
}

func applyPatch(plugin *models.Plugin, patch models.PluginPatch) {
	if patch.Namespace != nil {
		plugin.Namespace = *patch.Namespace
	}
	if patch.Name != nil {
		plugin.Name = *patch.Name
	}
	if patch.Description != nil {
		plugin.Description = *patch.Description
	}
	if patch.Author != nil {
		plugin.Author = *patch.Author
	}
	if patch.Category != nil {
		plugin.Category = *patch.Category
	}
	if patch.Repository != nil {
		plugin.Repository = *patch.Repository
	}
	if patch.License != nil {
		plugin.License = *patch.License
	}
	if patch.Tags != nil {
		plugin.Tags = *patch.Tags
	}
}
