package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Repository ownership verification.
//
// Until now anyone signed in could submit any repository under their own name.
// The author field was forced to the submitter's login, which records *who
// submitted* but proves nothing about whether they control what they submitted
// — so a plugin could be claimed out from under its actual maintainer, and a
// namespace entry could be created by someone with no connection to it.
//
// Verification uses only public GitHub endpoints, in increasing order of effort
// for the submitter:
//
//  1. They own the repository account (owner == login).
//  2. They are a public member of the owning organisation.
//  3. They put a claim file in the repository naming themselves.
//
// The claim file is the fallback that always works: a private org membership,
// a collaborator who is not a member, a repo owned by a bot account. It is the
// same mechanism a domain-verification check uses, and it only needs the write
// access a maintainer already has.

// ClaimFilePath is the file a submitter adds to prove they control a repository.
const ClaimFilePath = ".semrel-registry-claim"

// OwnershipResult explains how a claim was settled, so the caller can tell the
// submitter what to do next rather than only that they may not proceed.
type OwnershipResult struct {
	Verified bool   `json:"verified"`
	Method   string `json:"method,omitempty"`
	Issue    string `json:"issue,omitempty"`
	// HowToFix is shown to a submitter whose claim did not verify.
	HowToFix string `json:"howToFix,omitempty"`
}

// VerifyRepositoryOwnership checks whether login controls owner/repo.
func VerifyRepositoryOwnership(ctx context.Context, login, owner, repo string) OwnershipResult {
	login = strings.TrimSpace(login)
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)

	if login == "" || owner == "" || repo == "" {
		return OwnershipResult{Issue: "submitter and repository are both required"}
	}

	// 1. The repository lives under the submitter's own account.
	if strings.EqualFold(login, owner) {
		return OwnershipResult{Verified: true, Method: "account-owner"}
	}

	// 2. The submitter is a public member of the owning organisation.
	if isPublicOrgMember(ctx, owner, login) {
		return OwnershipResult{Verified: true, Method: "public-org-member"}
	}

	// 3. A claim file in the repository names the submitter.
	switch claimed, err := claimFileNames(ctx, owner, repo, login); {
	case err != nil:
		return OwnershipResult{
			Issue:    fmt.Sprintf("could not read %s: %v", ClaimFilePath, err),
			HowToFix: claimInstructions(login),
		}
	case claimed:
		return OwnershipResult{Verified: true, Method: "claim-file"}
	}

	return OwnershipResult{
		Issue:    fmt.Sprintf("%s does not own %s/%s, is not a public member of %s, and the repository carries no claim for them", login, owner, repo, owner),
		HowToFix: claimInstructions(login),
	}
}

func claimInstructions(login string) string {
	return fmt.Sprintf(
		"Add a file named %s to the default branch containing exactly %q, then submit again. "+
			"Alternatively, make your organisation membership public on GitHub.",
		ClaimFilePath, login)
}

// isPublicOrgMember uses the endpoint that answers 204 for a public member and
// 404 otherwise. It needs no authentication and discloses nothing private.
func isPublicOrgMember(ctx context.Context, org, login string) bool {
	_, status, err := ghRequest(ctx, fmt.Sprintf(
		"https://api.github.com/orgs/%s/public_members/%s", org, login))
	return err == nil && status == http.StatusNoContent
}

// claimFileNames reports whether the repository's claim file names login.
//
// The comparison is on the trimmed contents so that a trailing newline — which
// every editor adds — does not fail an otherwise correct claim.
func claimFileNames(ctx context.Context, owner, repo, login string) (bool, error) {
	body, status, err := ghRequest(ctx, fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s", owner, repo, ClaimFilePath))
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("github returned HTTP %d", status)
	}

	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Size     int    `json:"size"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, err
	}
	// A claim is one line. Anything larger is not one, and decoding it would
	// only give an attacker a way to make the registry read a large file.
	if payload.Size > 4096 {
		return false, fmt.Errorf("%s is too large to be a claim", ClaimFilePath)
	}

	contents, err := decodeGitHubContent(payload.Content, payload.Encoding)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(contents, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), login) {
			return true, nil
		}
	}
	return false, nil
}

// decodeGitHubContent unwraps the base64 the contents API returns. GitHub wraps
// the payload at 60 characters, which the standard decoder rejects.
func decodeGitHubContent(content, encoding string) (string, error) {
	if encoding != "" && encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding %q", encoding)
	}
	clean := strings.NewReplacer("\n", "", "\r", "").Replace(content)
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// POST /api/v1/plugins/verify-ownership
//
// Lets the submission form check a repository before the submitter fills in the
// rest of the form, so a failed claim is caught where it can still be fixed
// rather than at the end.
func VerifyOwnership(c *gin.Context) {
	var body struct {
		Repository string `json:"repository"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	login, _ := c.Get("login")
	loginStr, _ := login.(string)

	owner, repo := ownerRepoFromURL(body.Repository)
	if owner == "" || repo == "" {
		BadRequest(c, "A GitHub repository URL is required", nil)
		return
	}

	result := VerifyRepositoryOwnership(c.Request.Context(), loginStr, owner, repo)
	c.JSON(http.StatusOK, gin.H{"data": result})
}
