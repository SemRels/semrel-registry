package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func statementFor(subjectDigest, repository, workflowPath string) string {
	statement := map[string]any{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject": []map[string]any{{
			"name":   "plugin-linux-amd64",
			"digest": map[string]string{"sha256": subjectDigest},
		}},
		"predicate": map[string]any{
			"buildDefinition": map[string]any{
				"buildType": "https://actions.github.io/buildtypes/workflow/v1",
				"externalParameters": map[string]any{
					"workflow": map[string]any{
						"repository": "https://github.com/" + repository,
						"path":       workflowPath,
						"ref":        "refs/heads/main",
					},
				},
			},
		},
	}
	encoded, _ := json.Marshal(statement)
	return string(encoded)
}

func TestProvenanceAcceptsAMatchingAttestation(t *testing.T) {
	bare := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	statement := statementFor(bare, "SemRels/provider-github", ".github/workflows/release.yml")

	result, ok := readProvenanceStatement(statement, testDigest, "SemRels/provider-github")

	require.True(t, ok)
	assert.True(t, result.Verified)
	assert.Equal(t, "SemRels/provider-github", result.SourceRepository)
	assert.Equal(t, "release.yml", result.Workflow)
	assert.Equal(t, "https://slsa.dev/provenance/v1", result.PredicateType)
}

// The subject is the binding between an attestation and these exact bytes.
// Without checking it, any attestation in the repository would "verify" any
// artifact — which is the whole point of the check.
func TestProvenanceRejectsAnAttestationForOtherBytes(t *testing.T) {
	other := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	statement := statementFor(other, "SemRels/provider-github", ".github/workflows/release.yml")

	_, ok := readProvenanceStatement(statement, testDigest, "SemRels/provider-github")

	assert.False(t, ok, "an attestation covering different bytes must not match")
}

// The most interesting negative result: the artifact is attested, but by a
// repository the plugin does not claim to come from.
func TestProvenanceReportsAForeignBuilder(t *testing.T) {
	bare := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	statement := statementFor(bare, "attacker/lookalike", ".github/workflows/release.yml")

	result, ok := readProvenanceStatement(statement, testDigest, "SemRels/provider-github")

	require.True(t, ok)
	assert.False(t, result.Verified)
	assert.Contains(t, result.Issue, "attacker/lookalike")
	assert.Contains(t, result.Issue, "SemRels/provider-github")
}

func TestProvenanceIgnoresUnparseableStatements(t *testing.T) {
	_, ok := readProvenanceStatement("not json at all", testDigest, "SemRels/x")
	assert.False(t, ok)
}

func TestNormalizeDigest(t *testing.T) {
	bare := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	assert.Equal(t, testDigest, normalizeDigest(bare), "bare hex gains the prefix")
	assert.Equal(t, testDigest, normalizeDigest(testDigest), "prefixed input is unchanged")
	assert.Equal(t, testDigest, normalizeDigest("  SHA256:"+bare+"  "), "case and padding are tolerated")

	for name, input := range map[string]string{
		"empty":     "",
		"too short": "abc123",
		"not hex":   "zzzzc44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			assert.Empty(t, normalizeDigest(input))
		})
	}
}

// A missing digest or repository must produce a stated reason, never a silent
// "unverified" the reader cannot act on.
func TestVerifyProvenanceExplainsMissingInputs(t *testing.T) {
	assert.Contains(t, VerifyProvenance(context.Background(), "SemRels", "x", "not-a-digest").Issue, "digest")
	assert.Contains(t, VerifyProvenance(context.Background(), "", "", testDigest).Issue, "repository")
}

func TestDecodeStatementRoundTrip(t *testing.T) {
	// The attestation payload arrives base64-encoded inside the DSSE envelope.
	statement := statementFor("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "SemRels/x", "release.yml")
	encoded := base64.StdEncoding.EncodeToString([]byte(statement))

	decoded, err := decodeGitHubContent(encoded, "base64")

	require.NoError(t, err)
	assert.Equal(t, statement, decoded)
}
