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
	ID            int64           `json:"id"`
	Namespace     string          `json:"namespace,omitempty"`
	Name          string          `json:"name"`
	Aliases       []string        `json:"aliases,omitempty"`
	Description   string          `json:"description"`
	Author        string          `json:"author"`
	Category      string          `json:"category"`
	Repository    string          `json:"repository"`
	License       string          `json:"license"`
	Status        string          `json:"status"`
	Tags          []string        `json:"tags,omitempty"`
	Versions      []PluginVersion `json:"versions,omitempty"`
	LatestVersion string          `json:"latestVersion,omitempty"`
	// LatestSemrelCore is the core compatibility range of LatestVersion, so a
	// listing can show compatibility without fetching every plugin's versions.
	LatestSemrelCore string          `json:"latestSemrelCore,omitempty"`
	Views            int64           `json:"views"`
	Downloads        int64           `json:"downloads"`
	ValidationChecks json.RawMessage `json:"validationChecks,omitempty"`
	ValidatedAt      *time.Time      `json:"validatedAt,omitempty"`

	// SecurityAdvisories is imported from GitHub's own security advisories for
	// the plugin's repository. Nil means it has never been looked up; an empty
	// (non-nil) slice means it was checked and nothing was found.
	SecurityAdvisories  []SecurityAdvisory `json:"securityAdvisories,omitempty"`
	AdvisoriesCheckedAt *time.Time         `json:"advisoriesCheckedAt,omitempty"`

	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	DeletedBy      string     `json:"deletedBy,omitempty"`
	DeletionReason string     `json:"deletionReason,omitempty"`

	// Review outcome. A rejected submission used to carry no explanation, so
	// the author saw a "rejected" badge with nothing to act on.
	RejectionReason string     `json:"rejectionReason,omitempty"`
	ReviewedAt      *time.Time `json:"reviewedAt,omitempty"`
	ReviewedBy      string     `json:"reviewedBy,omitempty"`
}

// ReviewDecision is the body for approving or rejecting a submission.
type ReviewDecision struct {
	Reason string `json:"reason"`
}

type PluginVersion struct {
	ID             int64             `json:"id"`
	PluginID       int64             `json:"pluginId"`
	Version        string            `json:"version"`
	ReleaseDate    *time.Time        `json:"releaseDate,omitempty"`
	Changelog      string            `json:"changelog"`
	DownloadURL    string            `json:"downloadUrl"`
	Checksums      map[string]string `json:"checksums,omitempty"`
	SemrelCore     string            `json:"semrelCore,omitempty"`
	Prerelease     bool              `json:"prerelease"`
	Views          int64             `json:"views"`
	Downloads      int64             `json:"downloads"`
	CreatedAt      time.Time         `json:"createdAt"`
	DeletedAt      *time.Time        `json:"deletedAt,omitempty"`
	DeletedBy      string            `json:"deletedBy,omitempty"`
	DeletionReason string            `json:"deletionReason,omitempty"`

	// Provenance is recorded when the artifact digest can be matched to a build
	// attestation. Nil means it has not been looked up yet, which is different
	// from having been looked up and not found.
	Provenance          *Provenance `json:"provenance,omitempty"`
	ProvenanceCheckedAt *time.Time  `json:"provenanceCheckedAt,omitempty"`

	// Yank marks a version as unfit for new installs without removing it.
	// Existing pins keep resolving; `latest` skips it.
	YankedAt     *time.Time `json:"yankedAt,omitempty"`
	YankedBy     string     `json:"yankedBy,omitempty"`
	YankedReason string     `json:"yankedReason,omitempty"`
}

// Yanked reports whether this version has been retracted.
func (v PluginVersion) Yanked() bool {
	return v.YankedAt != nil
}

// VersionYankRequest is the body for yanking or un-yanking a version.
type VersionYankRequest struct {
	// Reason is shown to anyone who has the version pinned, so it is required:
	// "yanked" without a cause leaves consumers unable to judge the urgency.
	Reason string `json:"reason"`
}

type PluginPatch struct {
	Namespace   *string   `json:"namespace"`
	Name        *string   `json:"name"`
	Aliases     *[]string `json:"aliases"`
	Description *string   `json:"description"`
	Author      *string   `json:"author"`
	Category    *string   `json:"category"`
	Repository  *string   `json:"repository"`
	License     *string   `json:"license"`
	Tags        *[]string `json:"tags"`
}

type DeleteActor struct {
	Login   string
	Role    string
	IsAdmin bool
}

type PluginDeletionRequest struct {
	Confirmation   string `json:"confirmation"`
	Reason         string `json:"reason"`
	DeleteVersions bool   `json:"deleteVersions"`
}

type PluginDeletionSpec struct {
	PluginID        int64
	DeletedBy       string
	Reason          string
	Confirmation    string
	CascadeVersions bool
	AnonymizeAuthor bool
}

type VersionDeletionRequest struct {
	Confirmation string `json:"confirmation"`
	Reason       string `json:"reason"`
}

type VersionDeletionSpec struct {
	PluginID     int64
	VersionID    int64
	DeletedBy    string
	Reason       string
	Confirmation string
}

type AccountDeletionRequest struct {
	Confirmation       string `json:"confirmation"`
	Reason             string `json:"reason"`
	ReauthToken        string `json:"reauthToken"`
	DeleteOwnedPlugins bool   `json:"deleteOwnedPlugins"`
}

type AccountDeletionResult struct {
	PluginsDeleted  int `json:"pluginsDeleted"`
	VersionsDeleted int `json:"versionsDeleted"`
}

type AccountDeletionAudit struct {
	Login           string    `json:"login"`
	DeletedBy       string    `json:"deletedBy"`
	Reason          string    `json:"reason,omitempty"`
	Confirmation    string    `json:"confirmation"`
	PluginsDeleted  int       `json:"pluginsDeleted"`
	VersionsDeleted int       `json:"versionsDeleted"`
	CreatedAt       time.Time `json:"createdAt"`
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
		p.Aliases == nil &&
		p.Description == nil &&
		p.Author == nil &&
		p.Category == nil &&
		p.Repository == nil &&
		p.License == nil &&
		p.Tags == nil
}

// VersionYankSpec describes a yank or un-yank of a published version.
type VersionYankSpec struct {
	PluginID  int64
	VersionID int64
	// Yanked false lifts a previous yank.
	Yanked bool
	Actor  string
	Reason string
}

// ReviewOutcomeSpec records the result of reviewing a submitted plugin.
type ReviewOutcomeSpec struct {
	PluginID int64
	Status   string
	Reviewer string
	Reason   string
}

// PluginSubmission is a community submission: the plugin itself plus the
// contact details the submitter chose to provide.
//
// The address is a separate field rather than one on Plugin so that it cannot
// be serialised by accident. Plugin is returned by public endpoints and written
// into plugins.json; an address on that struct would be one forgotten `omit`
// away from being published.
type PluginSubmission struct {
	Plugin
	// NotifyEmail is optional. When set, the registry emails the review
	// outcome here and nowhere else.
	NotifyEmail string `json:"notifyEmail,omitempty"`
}

// ReviewNotification is what the notifier needs to tell an author what
// happened to their submission.
type ReviewNotification struct {
	Recipient  string
	PluginRef  string
	Repository string
	Approved   bool
	Reason     string
	Reviewer   string
}

// Provenance records where a published artifact was built.
//
// A checksum proves the bytes are intact; it cannot prove who produced them.
// This is the answer to the second question, taken from the attestation GitHub
// records when a workflow builds a release artifact.
type Provenance struct {
	// Verified is true when an attestation was found for the artifact digest
	// and it names the repository the plugin claims to come from.
	Verified bool `json:"verified"`
	// SourceRepository is the repository the attestation says built it, in
	// owner/name form. A mismatch with the plugin's own repository is the
	// interesting case: it means the artifact came from somewhere else.
	SourceRepository string `json:"sourceRepository,omitempty"`
	// Workflow is the build definition that produced the artifact.
	Workflow string `json:"workflow,omitempty"`
	// Digest is the artifact digest the attestation covers, "sha256:…".
	Digest string `json:"digest,omitempty"`
	// PredicateType names the attestation format, e.g. the SLSA provenance URI.
	PredicateType string `json:"predicateType,omitempty"`
	// Issue explains a negative result, so "unverified" is never mute.
	Issue string `json:"issue,omitempty"`
}

// SecurityAdvisory is a known vulnerability affecting a plugin, imported from
// the GitHub Security Advisories published on the plugin's own repository.
type SecurityAdvisory struct {
	GHSAID   string `json:"ghsaId"`
	CVEID    string `json:"cveId,omitempty"`
	Summary  string `json:"summary"`
	Severity string `json:"severity,omitempty"`
	URL      string `json:"url,omitempty"`
	// VulnerableRange is the affected versions, converted to the range syntax
	// this registry's own semver package understands (see ParseRange).
	VulnerableRange string     `json:"vulnerableRange,omitempty"`
	PatchedVersion  string     `json:"patchedVersion,omitempty"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty"`
	// WithdrawnAt marks an advisory GitHub itself retracted, e.g. a false
	// positive. It is imported but excluded from audit results.
	WithdrawnAt *time.Time `json:"withdrawnAt,omitempty"`
}

// Withdrawn reports whether GitHub itself retracted this advisory.
func (a SecurityAdvisory) Withdrawn() bool {
	return a.WithdrawnAt != nil
}
