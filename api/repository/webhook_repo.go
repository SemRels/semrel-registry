package repository

import (
	"context"
	"fmt"

	"github.com/SemRels/semrel-registry/api/database"
	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/jackc/pgx/v5"
)

// maxConsecutiveWebhookFailures is how many delivery failures in a row
// disable a subscription automatically. Without a cutoff, a consumer whose
// server has gone away forever would still receive (and fail) a delivery
// attempt for every future event, indefinitely.
const maxConsecutiveWebhookFailures = 10

// WebhookRepository stores consumer webhook subscriptions. It is separate
// from PluginRepository because a subscription is owned by whoever created
// it and can span any plugin, not data that belongs to one plugin record.
type WebhookRepository interface {
	Create(ctx context.Context, sub *models.WebhookSubscription) (int64, error)
	ListByOwner(ctx context.Context, owner string) ([]models.WebhookSubscription, error)
	// ListActiveForEvent returns active subscriptions that requested this
	// event for this plugin ref.
	ListActiveForEvent(ctx context.Context, pluginRef, event string) ([]models.WebhookSubscription, error)
	// Delete removes a subscription. Only its owner or an admin may delete it.
	Delete(ctx context.Context, id int64, owner string, isAdmin bool) error
	// RecordDelivery updates delivery bookkeeping after an attempt, disabling
	// the subscription once it crosses the consecutive-failure limit.
	RecordDelivery(ctx context.Context, id int64, statusCode int, success bool) error
}

type pgWebhookRepository struct {
	db *database.Database
}

func NewWebhookRepository(db *database.Database) WebhookRepository {
	return &pgWebhookRepository{db: db}
}

func (r *pgWebhookRepository) Create(ctx context.Context, sub *models.WebhookSubscription) (int64, error) {
	if sub == nil {
		return 0, fmt.Errorf("subscription is required")
	}
	var id int64
	err := r.db.Pool().QueryRow(ctx, `
INSERT INTO webhook_subscriptions (created_by, url, secret, plugin_ref, events, active)
VALUES ($1, $2, $3, $4, $5, true)
RETURNING id`, sub.CreatedBy, sub.URL, sub.Secret, sub.PluginRef, sub.Events).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create webhook subscription: %w", err)
	}
	return id, nil
}

func (r *pgWebhookRepository) ListByOwner(ctx context.Context, owner string) ([]models.WebhookSubscription, error) {
	rows, err := r.db.Pool().Query(ctx, `
SELECT id, created_by, url, plugin_ref, events, active, created_at, last_delivery_at, COALESCE(last_status_code, 0), consecutive_failures
FROM webhook_subscriptions
WHERE created_by = $1
ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, fmt.Errorf("list webhook subscriptions: %w", err)
	}
	defer rows.Close()
	return scanWebhookSubscriptions(rows)
}

func (r *pgWebhookRepository) ListActiveForEvent(ctx context.Context, pluginRef, event string) ([]models.WebhookSubscription, error) {
	rows, err := r.db.Pool().Query(ctx, `
SELECT id, created_by, url, secret, plugin_ref, events, active, created_at, last_delivery_at, COALESCE(last_status_code, 0), consecutive_failures
FROM webhook_subscriptions
WHERE active AND plugin_ref = $1 AND $2 = ANY(events)`, pluginRef, event)
	if err != nil {
		return nil, fmt.Errorf("list webhook subscriptions for event: %w", err)
	}
	defer rows.Close()

	var subs []models.WebhookSubscription
	for rows.Next() {
		var s models.WebhookSubscription
		if err := rows.Scan(&s.ID, &s.CreatedBy, &s.URL, &s.Secret, &s.PluginRef, &s.Events, &s.Active,
			&s.CreatedAt, &s.LastDeliveryAt, &s.LastStatusCode, &s.ConsecutiveFailures); err != nil {
			return nil, fmt.Errorf("scan webhook subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *pgWebhookRepository) Delete(ctx context.Context, id int64, owner string, isAdmin bool) error {
	query := `DELETE FROM webhook_subscriptions WHERE id = $1 AND created_by = $2`
	args := []interface{}{id, owner}
	if isAdmin {
		query = `DELETE FROM webhook_subscriptions WHERE id = $1`
		args = []interface{}{id}
	}
	tag, err := r.db.Pool().Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete webhook subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return appErrors.ErrPluginNotFound
	}
	return nil
}

func (r *pgWebhookRepository) RecordDelivery(ctx context.Context, id int64, statusCode int, success bool) error {
	var query string
	if success {
		query = `
UPDATE webhook_subscriptions
SET last_delivery_at = NOW(), last_status_code = $2, consecutive_failures = 0
WHERE id = $1`
	} else {
		query = fmt.Sprintf(`
UPDATE webhook_subscriptions
SET last_delivery_at = NOW(), last_status_code = $2, consecutive_failures = consecutive_failures + 1,
    active = (consecutive_failures + 1 < %d)
WHERE id = $1`, maxConsecutiveWebhookFailures)
	}
	tag, err := r.db.Pool().Exec(ctx, query, id, statusCode)
	if err != nil {
		return fmt.Errorf("record webhook delivery: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return appErrors.ErrPluginNotFound
	}
	return nil
}

func scanWebhookSubscriptions(rows pgx.Rows) ([]models.WebhookSubscription, error) {
	var subs []models.WebhookSubscription
	for rows.Next() {
		var s models.WebhookSubscription
		if err := rows.Scan(&s.ID, &s.CreatedBy, &s.URL, &s.PluginRef, &s.Events, &s.Active,
			&s.CreatedAt, &s.LastDeliveryAt, &s.LastStatusCode, &s.ConsecutiveFailures); err != nil {
			return nil, fmt.Errorf("scan webhook subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
