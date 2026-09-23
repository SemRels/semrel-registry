package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingNotifier captures what would have been delivered.
type recordingNotifier struct {
	sent []models.ReviewNotification
	err  error
}

func (r *recordingNotifier) NotifyReview(_ context.Context, n models.ReviewNotification) error {
	r.sent = append(r.sent, n)
	return r.err
}

func notifyingService(t *testing.T) (*PluginService, repository.PluginRepository, *recordingNotifier) {
	t.Helper()
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	notifier := &recordingNotifier{}
	return NewPluginServiceWithNotifier(repo, notifier), repo, notifier
}

func submissionOf(email string) models.PluginSubmission {
	return models.PluginSubmission{
		Plugin: models.Plugin{
			Name:       "analyzer-example",
			Category:   "analyzer",
			Author:     "alice",
			Repository: "https://github.com/alice/analyzer-example",
		},
		NotifyEmail: email,
	}
}

func TestReviewNotifiesTheSubmitterWhoAskedToBeTold(t *testing.T) {
	svc, _, notifier := notifyingService(t)
	ctx := context.Background()

	submitted, err := svc.SubmitPluginWithContact(ctx, submissionOf("alice@example.com"))
	require.NoError(t, err)

	_, err = svc.ReviewPlugin(ctx, submitted.Ref(), models.StatusRejected,
		models.ReviewDecision{Reason: "Missing a release workflow."}, "maintainer")
	require.NoError(t, err)

	require.Len(t, notifier.sent, 1)
	assert.Equal(t, "alice@example.com", notifier.sent[0].Recipient)
	assert.False(t, notifier.sent[0].Approved)
	assert.Equal(t, "Missing a release workflow.", notifier.sent[0].Reason)
}

func TestReviewStaysSilentWithoutAnAddress(t *testing.T) {
	svc, _, notifier := notifyingService(t)
	ctx := context.Background()

	submitted, err := svc.SubmitPluginWithContact(ctx, submissionOf(""))
	require.NoError(t, err)

	_, err = svc.ReviewPlugin(ctx, submitted.Ref(), models.StatusActive, models.ReviewDecision{}, "maintainer")
	require.NoError(t, err)

	assert.Empty(t, notifier.sent, "no address was given, so there is nobody to tell")
}

// A relay being down must not make a maintainer believe their decision failed.
func TestReviewSucceedsWhenDeliveryFails(t *testing.T) {
	svc, _, notifier := notifyingService(t)
	notifier.err = assert.AnError
	ctx := context.Background()

	submitted, err := svc.SubmitPluginWithContact(ctx, submissionOf("alice@example.com"))
	require.NoError(t, err)

	reviewed, err := svc.ReviewPlugin(ctx, submitted.Ref(), models.StatusActive, models.ReviewDecision{}, "maintainer")
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, reviewed.Status)
}

func TestSubmissionRejectsAMalformedAddress(t *testing.T) {
	svc, _, _ := notifyingService(t)

	_, err := svc.SubmitPluginWithContact(context.Background(), submissionOf("not-an-address"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notifyEmail")
}

// The address is personal data attached to an account that no longer exists.
func TestAccountDeletionErasesTheContactAddress(t *testing.T) {
	svc, repo, _ := notifyingService(t)
	ctx := context.Background()

	submitted, err := svc.SubmitPluginWithContact(ctx, submissionOf("alice@example.com"))
	require.NoError(t, err)

	stored, err := repo.NotifyEmail(ctx, submitted.ID)
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", stored)

	_, err = svc.DeleteAccount(ctx, models.AccountDeletionRequest{
		Confirmation:       "DELETE alice",
		ReauthToken:        "verified-by-the-handler",
		DeleteOwnedPlugins: true,
	}, models.DeleteActor{Login: "alice"})
	require.NoError(t, err)

	remaining, err := repo.NotifyEmail(ctx, submitted.ID)
	require.NoError(t, err)
	assert.Empty(t, remaining, "a deleted account must leave no contact address behind")
}

// The plugin record is what public endpoints and the catalogue export read, so
// the address must not be reachable through it at all.
func TestContactAddressIsNotPartOfThePluginRecord(t *testing.T) {
	svc, _, _ := notifyingService(t)
	ctx := context.Background()

	submitted, err := svc.SubmitPluginWithContact(ctx, submissionOf("alice@example.com"))
	require.NoError(t, err)

	fetched, err := svc.GetPlugin(ctx, submitted.Ref())
	require.NoError(t, err)

	encoded := mustJSON(t, fetched)
	assert.NotContains(t, encoded, "alice@example.com")
	assert.NotContains(t, encoded, "notifyEmail")
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
