package service

import (
	"context"
	"fmt"
	"strings"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
)

// maxReviewReasonLength bounds the free-text explanation shown to authors.
const maxReviewReasonLength = 1000

// ReviewPlugin records an approval or rejection together with its reason.
//
// A rejection reason is mandatory: a plugin author who sees only a "rejected"
// badge has nothing to fix and no way to know whether to try again. Approvals
// may carry a note but do not require one.
func (s *PluginService) ReviewPlugin(ctx context.Context, ref, status string, decision models.ReviewDecision, reviewer string) (models.Plugin, error) {
	if status != models.StatusActive && status != models.StatusRejected {
		return models.Plugin{}, &appErrors.ValidationError{Field: "status", Issue: "must be active or rejected"}
	}

	reason := strings.TrimSpace(decision.Reason)
	if status == models.StatusRejected && reason == "" {
		return models.Plugin{}, &appErrors.ValidationError{
			Field: "reason",
			Issue: "is required when rejecting a submission, so the author knows what to change",
		}
	}
	if len(reason) > maxReviewReasonLength {
		return models.Plugin{}, &appErrors.ValidationError{
			Field: "reason",
			Issue: fmt.Sprintf("must be at most %d characters", maxReviewReasonLength),
		}
	}

	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.Plugin{}, err
	}

	if err := s.repo.SetReviewOutcome(ctx, models.ReviewOutcomeSpec{
		PluginID: plugin.ID,
		Status:   status,
		Reviewer: reviewer,
		Reason:   reason,
	}); err != nil {
		return models.Plugin{}, err
	}

	updated, err := s.repo.GetByID(ctx, plugin.ID)
	if err != nil {
		return models.Plugin{}, err
	}

	// The author asked to be told, so tell them. Until now a decision was
	// recorded and the author had to think to come back and look.
	s.notifyReviewOutcome(ctx, *updated, status == models.StatusActive, reason, reviewer)

	return *updated, nil
}

// YankVersion retracts a published version, or lifts the retraction.
//
// Retraction rather than deletion is the point: deleting a version breaks every
// build that already pins it, while a yanked version stays resolvable for those
// builds and disappears only from version resolution for new installs.
func (s *PluginService) YankVersion(ctx context.Context, ref string, versionID int64, yanked bool, request models.VersionYankRequest, actor models.DeleteActor) (models.PluginVersion, error) {
	reason := strings.TrimSpace(request.Reason)
	if yanked && reason == "" {
		return models.PluginVersion{}, &appErrors.ValidationError{
			Field: "reason",
			Issue: "is required when yanking a version, so consumers can judge the urgency",
		}
	}
	if len(reason) > maxReviewReasonLength {
		return models.PluginVersion{}, &appErrors.ValidationError{
			Field: "reason",
			Issue: fmt.Sprintf("must be at most %d characters", maxReviewReasonLength),
		}
	}

	plugin, err := s.lookupPlugin(ctx, ref)
	if err != nil {
		return models.PluginVersion{}, err
	}

	if err := s.repo.SetVersionYank(ctx, models.VersionYankSpec{
		PluginID:  plugin.ID,
		VersionID: versionID,
		Yanked:    yanked,
		Actor:     actor.Login,
		Reason:    reason,
	}); err != nil {
		return models.PluginVersion{}, err
	}

	versions, err := s.repo.GetVersions(ctx, plugin.ID)
	if err != nil {
		return models.PluginVersion{}, err
	}
	for _, version := range versions {
		if version.ID == versionID {
			return version, nil
		}
	}
	return models.PluginVersion{}, appErrors.ErrPluginNotFound
}

// LatestInstallableVersion returns the version a client should install by
// default: the newest stable release that has not been yanked.
func LatestInstallableVersion(versions []models.PluginVersion) (models.PluginVersion, bool) {
	for _, version := range versions {
		if version.Prerelease || version.Yanked() || version.DeletedAt != nil {
			continue
		}
		return version, true
	}
	return models.PluginVersion{}, false
}
