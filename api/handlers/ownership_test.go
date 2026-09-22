package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The cheapest case, and the only one that needs no network: the repository
// lives under the submitter's own account.
func TestOwnershipAcceptsTheAccountOwner(t *testing.T) {
	result := VerifyRepositoryOwnership("alice", "alice", "analyzer-example")

	assert.True(t, result.Verified)
	assert.Equal(t, "account-owner", result.Method)
}

func TestOwnershipIsCaseInsensitiveOnTheLogin(t *testing.T) {
	// GitHub logins are case-insensitive, and the session carries whatever
	// casing the profile uses.
	assert.True(t, VerifyRepositoryOwnership("Alice", "alice", "x").Verified)
	assert.True(t, VerifyRepositoryOwnership("alice", "ALICE", "x").Verified)
}

func TestOwnershipRequiresBothPartiesNamed(t *testing.T) {
	for name, args := range map[string][3]string{
		"no login": {"", "alice", "repo"},
		"no owner": {"alice", "", "repo"},
		"no repo":  {"alice", "owner", ""},
	} {
		t.Run(name, func(t *testing.T) {
			result := VerifyRepositoryOwnership(args[0], args[1], args[2])
			assert.False(t, result.Verified)
			assert.NotEmpty(t, result.Issue)
		})
	}
}

// A refusal has to tell the submitter what would make it succeed. Otherwise the
// check is a wall rather than a step.
func TestOwnershipRefusalExplainsTheFix(t *testing.T) {
	result := VerifyRepositoryOwnership("mallory", "some-unlikely-org-93ba7", "repo-93ba7")

	assert.False(t, result.Verified)
	assert.Contains(t, result.HowToFix, ClaimFilePath)
	assert.Contains(t, result.HowToFix, "mallory")
}

func TestDecodeGitHubContentUnwrapsWrappedBase64(t *testing.T) {
	// The contents API wraps base64 at 60 characters; the standard decoder
	// rejects the newlines.
	decoded, err := decodeGitHubContent("YWxp\nY2UK", "base64")

	assert.NoError(t, err)
	assert.Equal(t, "alice\n", decoded)
}

func TestDecodeGitHubContentRejectsUnknownEncoding(t *testing.T) {
	_, err := decodeGitHubContent("whatever", "none")
	assert.Error(t, err)
}
