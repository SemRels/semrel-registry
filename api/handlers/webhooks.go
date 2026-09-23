package handlers

// Consumer webhook subscriptions.
//
// The registry already accepts an inbound webhook: a plugin repository
// telling the registry about a new release. This is the other direction — a
// registry consumer, not necessarily the plugin's own publisher, asking to be
// told when something happens to a plugin they depend on, instead of polling
// plugins.json on a timer.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
)

// maxWebhookSubscriptionsPerOwner bounds how many endpoints one account can
// register, so a single account cannot turn the registry into a way to spam
// arbitrary URLs.
const maxWebhookSubscriptionsPerOwner = 20

var validWebhookEvents = func() map[string]bool {
	m := make(map[string]bool, len(models.WebhookEvents))
	for _, e := range models.WebhookEvents {
		m[e] = true
	}
	return m
}()

type WebhookHandler struct {
	repo    repository.WebhookRepository
	plugins service.PluginManager
}

func NewWebhookHandler(repo repository.WebhookRepository, plugins service.PluginManager) *WebhookHandler {
	return &WebhookHandler{repo: repo, plugins: plugins}
}

// CreateSubscription registers a new webhook subscription. The secret used to
// sign deliveries is returned once, in this response, and never again.
// POST /api/v1/webhooks/subscriptions
func (h *WebhookHandler) CreateSubscription(c *gin.Context) {
	var body models.WebhookSubscriptionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	if err := service.ValidateWebhookURL(c.Request.Context(), body.URL); err != nil {
		HandleError(c, err)
		return
	}
	if strings.TrimSpace(body.PluginRef) == "" {
		BadRequest(c, "pluginRef is required", nil)
		return
	}
	if _, err := h.plugins.GetPlugin(c.Request.Context(), body.PluginRef); err != nil {
		HandleError(c, err)
		return
	}
	if len(body.Events) == 0 {
		BadRequest(c, "at least one event is required", gin.H{"allowed": models.WebhookEvents})
		return
	}
	for _, e := range body.Events {
		if !validWebhookEvents[e] {
			BadRequest(c, "unknown event", gin.H{"event": e, "allowed": models.WebhookEvents})
			return
		}
	}

	login, _ := c.Get("login")
	loginStr, _ := login.(string)
	if loginStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	existing, err := h.repo.ListByOwner(c.Request.Context(), loginStr)
	if err != nil {
		HandleError(c, err)
		return
	}
	if len(existing) >= maxWebhookSubscriptionsPerOwner {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "subscription limit reached",
			"limit": maxWebhookSubscriptionsPerOwner,
		})
		return
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		InternalServerError(c, "failed to generate webhook secret", err)
		return
	}

	sub := &models.WebhookSubscription{
		CreatedBy: loginStr,
		URL:       body.URL,
		Secret:    secret,
		PluginRef: body.PluginRef,
		Events:    body.Events,
	}
	id, err := h.repo.Create(c.Request.Context(), sub)
	if err != nil {
		HandleError(c, err)
		return
	}

	// The only response that ever carries the secret — list/get responses
	// never include it, and the repository never returns it to ListByOwner.
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"id":        id,
		"createdBy": sub.CreatedBy,
		"url":       sub.URL,
		"secret":    secret,
		"pluginRef": sub.PluginRef,
		"events":    sub.Events,
		"active":    true,
	}})
}

// ListSubscriptions lists the caller's own subscriptions.
// GET /api/v1/webhooks/subscriptions
func (h *WebhookHandler) ListSubscriptions(c *gin.Context) {
	login, _ := c.Get("login")
	loginStr, _ := login.(string)
	subs, err := h.repo.ListByOwner(c.Request.Context(), loginStr)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": subs})
}

// DeleteSubscription removes one of the caller's own subscriptions; admins
// may remove any subscription.
// DELETE /api/v1/webhooks/subscriptions/:id
func (h *WebhookHandler) DeleteSubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid subscription id", gin.H{"issue": "must be an integer"})
		return
	}
	login, _ := c.Get("login")
	loginStr, _ := login.(string)
	isAdmin, _ := c.Get("isAdmin")

	if err := h.repo.Delete(c.Request.Context(), id, loginStr, isAdmin == true); err != nil {
		HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ── Delivery ────────────────────────────────────────────────────────────────

var webhookDeliveryClient = &http.Client{Timeout: 10 * time.Second}

// DeliverWebhookEvent notifies every active subscription for (pluginRef,
// event), signing the body the same way the registry itself requires of
// inbound release webhooks (HMAC-SHA256, X-Hub-Signature-256) so a consumer
// can verify it with tooling it may already have. It is fire-and-forget and
// never blocks or fails the caller; webhooks may be nil, in which case this
// is a no-op.
func DeliverWebhookEvent(webhooks repository.WebhookRepository, pluginRef, event string, data interface{}) {
	if webhooks == nil {
		return
	}
	go func() {
		subs, err := webhooks.ListActiveForEvent(context.Background(), pluginRef, event)
		if err != nil || len(subs) == 0 {
			return
		}
		payload := models.WebhookEventPayload{
			Event: event, PluginRef: pluginRef, Data: data, Timestamp: time.Now().UTC(),
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}
		for _, sub := range subs {
			go deliverWebhook(webhooks, sub, body)
		}
	}()
}

func deliverWebhook(webhooks repository.WebhookRepository, sub models.WebhookSubscription, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Re-validate immediately before dialing: DNS can change between when a
	// URL passed validation at subscription time and now.
	if err := service.ValidateWebhookURL(ctx, sub.URL); err != nil {
		_ = webhooks.RecordDelivery(context.Background(), sub.ID, 0, false)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	mac := hmac.New(sha256.New, []byte(sub.Secret))
	mac.Write(body)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))

	resp, err := webhookDeliveryClient.Do(req)
	status := 0
	success := false
	if err == nil && resp != nil {
		status = resp.StatusCode
		success = status >= 200 && status < 300
		_ = resp.Body.Close()
	}
	_ = webhooks.RecordDelivery(context.Background(), sub.ID, status, success)
}
