package models

import "time"

// Webhook event names a subscription can request. Unlike the registry's own
// inbound release webhook (a plugin repository telling the registry about a
// new release), these are outbound: the registry telling a consumer about
// something that happened to a plugin they depend on.
const (
	WebhookEventVersionPublished  = "version.published"
	WebhookEventVersionYanked     = "version.yanked"
	WebhookEventVersionUnyanked   = "version.unyanked"
	WebhookEventAdvisoryPublished = "advisory.published"
)

// WebhookEvents lists every event a subscription may request.
var WebhookEvents = []string{
	WebhookEventVersionPublished,
	WebhookEventVersionYanked,
	WebhookEventVersionUnyanked,
	WebhookEventAdvisoryPublished,
}

// WebhookSubscription lets a registry consumer — not just a plugin's own
// publisher — ask to be notified when something happens to a plugin they
// depend on, without polling plugins.json.
type WebhookSubscription struct {
	ID        int64  `json:"id"`
	CreatedBy string `json:"createdBy"`
	URL       string `json:"url"`
	// Secret signs delivered payloads (X-Hub-Signature-256, same scheme the
	// registry itself requires of inbound release webhooks). It is shown once,
	// in the create response, and is never serialized afterward.
	Secret    string    `json:"-"`
	PluginRef string    `json:"pluginRef"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`

	// A subscription is disabled automatically after too many consecutive
	// delivery failures, so a dead endpoint does not get hammered forever.
	LastDeliveryAt      *time.Time `json:"lastDeliveryAt,omitempty"`
	LastStatusCode      int        `json:"lastStatusCode,omitempty"`
	ConsecutiveFailures int        `json:"consecutiveFailures,omitempty"`
}

// WebhookSubscriptionRequest is the body for creating a subscription.
type WebhookSubscriptionRequest struct {
	URL       string   `json:"url"`
	PluginRef string   `json:"pluginRef"`
	Events    []string `json:"events"`
}

// WebhookEventPayload is what gets POSTed to a subscriber.
type WebhookEventPayload struct {
	Event     string      `json:"event"`
	PluginRef string      `json:"pluginRef"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}
