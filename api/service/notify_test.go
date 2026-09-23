package service

import (
	"context"
	"strings"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNotifyEmail(t *testing.T) {
	valid := []string{"", "  ", "alice@example.com", "a.b+tag@sub.example.co.uk"}
	for _, address := range valid {
		t.Run("accepts "+address, func(t *testing.T) {
			require.NoError(t, ValidateNotifyEmail(address))
		})
	}

	invalid := map[string]string{
		"no at sign":      "alice.example.com",
		"no domain dot":   "alice@localhost",
		"header newline":  "alice@example.com\nBcc: victim@example.com",
		"carriage return": "alice@example.com\rBcc: victim@example.com",
		"angle brackets":  "<alice@example.com>",
		"comma list":      "alice@example.com,bob@example.com",
		"too long":        strings.Repeat("a", 250) + "@example.com",
	}
	for name, address := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			require.Error(t, ValidateNotifyEmail(address))
		})
	}
}

// The plugin reference and the reviewer's reason are both user input, and a
// newline in a header is how a second message gets appended to the first.
func TestBuildMessageStripsHeaderInjection(t *testing.T) {
	message := string(buildMessage(
		"registry@example.com",
		"alice@example.com",
		"Subject\r\nBcc: victim@example.com",
		"Body text",
	))

	headers, _, found := strings.Cut(message, "\r\n\r\n")
	require.True(t, found, "message must separate headers from body")

	// The injected text survives as the *content* of the Subject, which is
	// harmless. What must not exist is a second header line carrying it.
	for _, line := range strings.Split(headers, "\r\n") {
		assert.False(t, strings.HasPrefix(line, "Bcc:"), "injected header line: %q", line)
	}
	assert.Contains(t, headers, "Subject: Subject")
}

func TestReviewMessageExplainsTheOutcome(t *testing.T) {
	approvedSubject, approvedBody := reviewMessage(models.ReviewNotification{
		PluginRef: "@semrel/analyzer-example",
		Approved:  true,
	}, "https://registry.semrel.io")

	assert.Contains(t, approvedSubject, "published")
	assert.Contains(t, approvedBody, "semrel plugin install @semrel/analyzer-example")

	rejectedSubject, rejectedBody := reviewMessage(models.ReviewNotification{
		PluginRef: "@semrel/analyzer-example",
		Approved:  false,
		Reason:    "The repository has no release workflow.",
	}, "https://registry.semrel.io")

	assert.Contains(t, rejectedSubject, "not accepted")
	// The reason is the point of the message: without it the author has
	// nothing to act on.
	assert.Contains(t, rejectedBody, "The repository has no release workflow.")
	assert.Contains(t, rejectedBody, "submit again")
}

// A missing recipient is not an error — most submissions provide no address.
func TestSMTPNotifierSkipsEmptyRecipient(t *testing.T) {
	notifier := NewSMTPNotifier(SMTPConfig{Host: "localhost", From: "registry@example.com"})
	require.NoError(t, notifier.NotifyReview(context.Background(), models.ReviewNotification{}))
}

func TestSMTPConfigRequiresHostAndSender(t *testing.T) {
	assert.False(t, SMTPConfig{}.Configured())
	assert.False(t, SMTPConfig{Host: "mail.example.com"}.Configured())
	assert.False(t, SMTPConfig{From: "registry@example.com"}.Configured())
	assert.True(t, SMTPConfig{Host: "mail.example.com", From: "registry@example.com"}.Configured())
}
