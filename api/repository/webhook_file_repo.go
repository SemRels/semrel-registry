package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
)

// fileWebhookRepository stores subscriptions in a single JSON file, the same
// shape as the file backend's notify-emails store: expected volume is a
// handful of subscriptions per deployment, not a table worth its own schema.
type fileWebhookRepository struct {
	mu   sync.Mutex
	path string
}

type webhookFile struct {
	NextID        int64                        `json:"nextId"`
	Subscriptions []models.WebhookSubscription `json:"subscriptions"`
}

func NewFileWebhookRepository(dataDir string) (WebhookRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("webhook repository: create data directory: %w", err)
	}
	return &fileWebhookRepository{path: filepath.Join(dataDir, "webhook-subscriptions.json")}, nil
}

func (r *fileWebhookRepository) load() (webhookFile, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return webhookFile{NextID: 1}, nil
	}
	if err != nil {
		return webhookFile{}, fmt.Errorf("read webhook subscriptions: %w", err)
	}
	var f webhookFile
	if err := json.Unmarshal(data, &f); err != nil {
		return webhookFile{}, fmt.Errorf("parse webhook subscriptions: %w", err)
	}
	if f.NextID == 0 {
		f.NextID = 1
	}
	return f, nil
}

func (r *fileWebhookRepository) save(f webhookFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal webhook subscriptions: %w", err)
	}
	return writeFileAtomicMode(r.path, data, 0o600)
}

func (r *fileWebhookRepository) Create(_ context.Context, sub *models.WebhookSubscription) (int64, error) {
	if sub == nil {
		return 0, fmt.Errorf("subscription is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return 0, err
	}
	sub.ID = f.NextID
	sub.Active = true
	f.NextID++
	f.Subscriptions = append(f.Subscriptions, *sub)
	if err := r.save(f); err != nil {
		return 0, err
	}
	return sub.ID, nil
}

func (r *fileWebhookRepository) ListByOwner(_ context.Context, owner string) ([]models.WebhookSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return nil, err
	}
	subs := make([]models.WebhookSubscription, 0)
	for _, s := range f.Subscriptions {
		if s.CreatedBy == owner {
			s.Secret = ""
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (r *fileWebhookRepository) ListActiveForEvent(_ context.Context, pluginRef, event string) ([]models.WebhookSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return nil, err
	}
	var subs []models.WebhookSubscription
	for _, s := range f.Subscriptions {
		if !s.Active || s.PluginRef != pluginRef {
			continue
		}
		for _, e := range s.Events {
			if e == event {
				subs = append(subs, s)
				break
			}
		}
	}
	return subs, nil
}

func (r *fileWebhookRepository) Delete(_ context.Context, id int64, owner string, isAdmin bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return err
	}
	for i, s := range f.Subscriptions {
		if s.ID != id {
			continue
		}
		if !isAdmin && s.CreatedBy != owner {
			return appErrors.ErrPluginNotFound
		}
		f.Subscriptions = append(f.Subscriptions[:i], f.Subscriptions[i+1:]...)
		return r.save(f)
	}
	return appErrors.ErrPluginNotFound
}

func (r *fileWebhookRepository) RecordDelivery(_ context.Context, id int64, statusCode int, success bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return err
	}
	for i := range f.Subscriptions {
		if f.Subscriptions[i].ID != id {
			continue
		}
		now := time.Now().UTC()
		f.Subscriptions[i].LastDeliveryAt = &now
		f.Subscriptions[i].LastStatusCode = statusCode
		if success {
			f.Subscriptions[i].ConsecutiveFailures = 0
		} else {
			f.Subscriptions[i].ConsecutiveFailures++
			if f.Subscriptions[i].ConsecutiveFailures >= maxConsecutiveWebhookFailures {
				f.Subscriptions[i].Active = false
			}
		}
		return r.save(f)
	}
	return appErrors.ErrPluginNotFound
}
