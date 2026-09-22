package service

import (
	"context"
	"log"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
)

// SubmitPluginWithContact submits a community plugin and, when the submitter
// provided one, records the address to notify about the review outcome.
//
// The address is stored separately from the plugin record. The plugin record is
// what public endpoints return and what the catalogue export reads, so keeping
// personal data out of it is a structural guarantee rather than a discipline.
func (s *PluginService) SubmitPluginWithContact(ctx context.Context, submission models.PluginSubmission) (models.Plugin, error) {
	if err := ValidateNotifyEmail(submission.NotifyEmail); err != nil {
		return models.Plugin{}, &appErrors.ValidationError{Field: "notifyEmail", Issue: err.Error()}
	}

	created, err := s.SubmitPlugin(ctx, submission.Plugin)
	if err != nil {
		return models.Plugin{}, err
	}

	if submission.NotifyEmail != "" {
		if err := s.repo.SetNotifyEmail(ctx, created.ID, submission.NotifyEmail); err != nil {
			// The submission itself succeeded. Failing it because the contact
			// address could not be stored would lose the author's work over a
			// convenience.
			log.Printf("could not store notification address for plugin %d: %v", created.ID, err)
		}
	}

	return created, nil
}

// notifyReviewOutcome delivers the review result to the submitter, if they
// asked to be told and a delivery channel is configured.
//
// Failures are logged, never returned: the review has already been recorded,
// and a mail relay being down is not a reason to tell a maintainer their
// decision did not go through.
func (s *PluginService) notifyReviewOutcome(ctx context.Context, plugin models.Plugin, approved bool, reason, reviewer string) {
	if s.notifier == nil {
		return
	}

	recipient, err := s.repo.NotifyEmail(ctx, plugin.ID)
	if err != nil {
		log.Printf("could not read notification address for plugin %d: %v", plugin.ID, err)
		return
	}
	if recipient == "" {
		return
	}

	if err := s.notifier.NotifyReview(ctx, models.ReviewNotification{
		Recipient:  recipient,
		PluginRef:  plugin.Ref(),
		Repository: plugin.Repository,
		Approved:   approved,
		Reason:     reason,
		Reviewer:   reviewer,
	}); err != nil {
		log.Printf("review notification for plugin %d failed: %v", plugin.ID, err)
	}
}
