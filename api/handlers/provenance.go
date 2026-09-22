package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
)

// Build provenance.
//
// The registry already stores a SHA-256 per platform, which proves the bytes
// were not altered in transit. It proves nothing about their origin: a
// publisher whose token has been stolen can compute a perfectly correct
// checksum for a malicious binary, and the registry would serve it happily.
//
// GitHub records an attestation when a workflow builds a release artifact, and
// exposes it publicly keyed by the artifact's digest. Looking the digest up
// therefore answers the question the checksum cannot: which repository and
// which workflow produced these exact bytes.
//
// This verifies the attestation's *subject* — that an attestation exists for
// this digest and names the expected repository. It does not verify the
// Sigstore signature bundle itself; that needs the full transparency-log
// client, and is the next step rather than this one. What it rules out is the
// case that matters most in a registry: an artifact whose bytes no build in the
// claimed repository ever produced.

// VerifyProvenance looks up the build attestation for an artifact digest.
//
// digest is the bare hex SHA-256 or a "sha256:"-prefixed one; expectedRepo is
// the plugin's own repository in owner/name form.
func VerifyProvenance(owner, repo, digest string) models.Provenance {
	normalized := normalizeDigest(digest)
	if normalized == "" {
		return models.Provenance{Issue: "no usable artifact digest to look up"}
	}
	if owner == "" || repo == "" {
		return models.Provenance{Digest: normalized, Issue: "the plugin has no GitHub repository on record"}
	}

	body, status, err := ghRequest(fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/attestations/%s", owner, repo, normalized))
	if err != nil {
		return models.Provenance{Digest: normalized, Issue: "could not reach GitHub: " + err.Error()}
	}
	if status == http.StatusNotFound {
		return models.Provenance{
			Digest: normalized,
			Issue:  "no build attestation is published for this artifact",
		}
	}
	if status != http.StatusOK {
		return models.Provenance{Digest: normalized, Issue: fmt.Sprintf("github returned HTTP %d", status)}
	}

	var payload struct {
		Attestations []struct {
			Bundle struct {
				DSSEEnvelope struct {
					// The statement is base64 in the envelope's payload.
					Payload     string `json:"payload"`
					PayloadType string `json:"payloadType"`
				} `json:"dsseEnvelope"`
			} `json:"bundle"`
		} `json:"attestations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return models.Provenance{Digest: normalized, Issue: "attestation response could not be read"}
	}
	if len(payload.Attestations) == 0 {
		return models.Provenance{Digest: normalized, Issue: "no build attestation is published for this artifact"}
	}

	expected := owner + "/" + repo
	for _, attestation := range payload.Attestations {
		statement, err := decodeGitHubContent(attestation.Bundle.DSSEEnvelope.Payload, "base64")
		if err != nil {
			continue
		}
		provenance, ok := readProvenanceStatement(statement, normalized, expected)
		if ok {
			return provenance
		}
	}

	return models.Provenance{
		Digest: normalized,
		Issue:  "an attestation exists but none of them covers this digest for " + expected,
	}
}

// readProvenanceStatement pulls the fields worth recording out of an in-toto
// statement, and confirms it actually covers the digest and repository claimed.
func readProvenanceStatement(statement, digest, expectedRepo string) (models.Provenance, bool) {
	var parsed struct {
		Type          string `json:"_type"`
		PredicateType string `json:"predicateType"`
		Subject       []struct {
			Name   string            `json:"name"`
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
		Predicate struct {
			BuildDefinition struct {
				BuildType          string `json:"buildType"`
				ExternalParameters struct {
					Workflow struct {
						Repository string `json:"repository"`
						Path       string `json:"path"`
						Ref        string `json:"ref"`
					} `json:"workflow"`
				} `json:"externalParameters"`
			} `json:"buildDefinition"`
		} `json:"predicate"`
	}
	if err := json.Unmarshal([]byte(statement), &parsed); err != nil {
		return models.Provenance{}, false
	}

	// The subject is the binding between the attestation and these exact bytes.
	// Without checking it, any attestation in the repository would "verify" any
	// artifact — which is the whole thing this is meant to prevent.
	covers := false
	for _, subject := range parsed.Subject {
		if strings.EqualFold("sha256:"+subject.Digest["sha256"], digest) {
			covers = true
			break
		}
	}
	if !covers {
		return models.Provenance{}, false
	}

	workflow := parsed.Predicate.BuildDefinition.ExternalParameters.Workflow
	sourceRepo := strings.TrimPrefix(workflow.Repository, "https://github.com/")

	provenance := models.Provenance{
		Digest:           digest,
		PredicateType:    parsed.PredicateType,
		SourceRepository: sourceRepo,
		Workflow:         strings.TrimPrefix(workflow.Path, ".github/workflows/"),
	}

	// An attestation from a different repository is not a pass. It is the most
	// interesting negative result there is: the bytes were built somewhere the
	// plugin does not claim to come from.
	if sourceRepo != "" && !strings.EqualFold(sourceRepo, expectedRepo) {
		provenance.Issue = fmt.Sprintf("built by %s, but the plugin claims %s", sourceRepo, expectedRepo)
		return provenance, true
	}

	provenance.Verified = true
	return provenance, true
}

// ReverifyProvenance re-runs the build-attestation lookup for one version on
// demand, synchronously — unlike the check on publish, someone asking for this
// is waiting for the answer, most often because the attestation did not exist
// yet the first time.
// POST /api/v1/plugins/:id/versions/:versionId/reverify-provenance
func (h *PluginHandler) ReverifyProvenance(c *gin.Context) {
	ref := c.Param("id")
	// The path segment is named :version, not :versionId — see the route
	// comment in main.go for why — but it still carries the numeric version id.
	versionID, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid version id", gin.H{"issue": "must be an integer"})
		return
	}

	plugin, err := h.service.GetPlugin(c.Request.Context(), ref)
	if err != nil {
		HandleError(c, err)
		return
	}

	// Same rule as every other write: publishers manage their own plugins,
	// admins manage all of them.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		loginStr, _ := login.(string)
		if !strings.EqualFold(plugin.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "you can only reverify versions of your own plugins",
				"author": plugin.Author,
			})
			return
		}
	}

	versions, err := h.service.ListVersions(c.Request.Context(), ref, 1000, 0)
	if err != nil {
		HandleError(c, err)
		return
	}
	var found *models.PluginVersion
	for i := range versions {
		if versions[i].ID == versionID {
			found = &versions[i]
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}

	digest := primaryDigest(found.Checksums)
	if digest == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "version has no checksum to look up an attestation for"})
		return
	}

	owner, repo := ownerRepoFromURL(plugin.Repository)
	provenance := VerifyProvenance(owner, repo, digest)
	if err := h.service.SetProvenance(c.Request.Context(), versionID, &provenance); err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": provenance})
}

// triggerProvenanceCheck looks up the build attestation for a newly published
// version in the background. It never blocks or fails the publish: GitHub
// being unreachable, or no attestation existing yet, just leaves provenance
// unset until the next publish or a manual recheck.
func triggerProvenanceCheck(svc service.PluginManager, versionID int64, repositoryURL string, checksums map[string]string) {
	digest := primaryDigest(checksums)
	if digest == "" {
		return
	}
	owner, repo := ownerRepoFromURL(repositoryURL)
	go func() {
		provenance := VerifyProvenance(owner, repo, digest)
		_ = svc.SetProvenance(context.Background(), versionID, &provenance)
	}()
}

// primaryDigest picks the checksum that corresponds to the artifact most
// likely to have an attestation worth checking — the same linux/amd64
// preference pickDownloadURL uses, so provenance is checked for the binary
// consumers actually get by default. Falls back to the lexicographically
// first entry so the choice is deterministic across runs.
func primaryDigest(checksums map[string]string) string {
	if digest := checksums["linux_amd64"]; digest != "" {
		return digest
	}
	keys := make([]string, 0, len(checksums))
	for k := range checksums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if checksums[k] != "" {
			return checksums[k]
		}
	}
	return ""
}

// normalizeDigest accepts both the bare hex the registry stores and the
// prefixed form the attestation API expects, and returns the prefixed form.
func normalizeDigest(digest string) string {
	trimmed := strings.ToLower(strings.TrimSpace(digest))
	trimmed = strings.TrimPrefix(trimmed, "sha256:")
	if len(trimmed) != 64 {
		return ""
	}
	for _, r := range trimmed {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return ""
		}
	}
	return "sha256:" + trimmed
}
