package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "webhook-secret-value-at-least-32-chars"

func webhookRequest(body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/release", strings.NewReader(body))
	for key, value := range headers {
		c.Request.Header.Set(key, value)
	}
	return c, recorder
}

func signBody(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookAcceptsValidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSyncHandlerWithSecret(nil, testWebhookSecret)
	body := `{"repository":"analyzer-conventional","tag":"v1.0.0"}`

	c, _ := webhookRequest(body, map[string]string{"X-Hub-Signature-256": signBody(testWebhookSecret, body)})

	require.NoError(t, handler.authenticateWebhook(c, []byte(body)))
}

// A signature covers the body, so an intermediary cannot repoint a legitimate
// release notification at a different repository.
func TestWebhookRejectsTamperedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSyncHandlerWithSecret(nil, testWebhookSecret)
	original := `{"repository":"analyzer-conventional","tag":"v1.0.0"}`
	tampered := `{"repository":"semrel-plugins","tag":"v1.0.0"}`

	c, _ := webhookRequest(tampered, map[string]string{"X-Hub-Signature-256": signBody(testWebhookSecret, original)})

	err := handler.authenticateWebhook(c, []byte(tampered))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

func TestWebhookRejectsMissingCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSyncHandlerWithSecret(nil, testWebhookSecret)
	body := `{"repository":"analyzer-conventional"}`

	c, _ := webhookRequest(body, nil)

	require.Error(t, handler.authenticateWebhook(c, []byte(body)))
}

func TestWebhookRejectsWrongSharedSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSyncHandlerWithSecret(nil, testWebhookSecret)
	body := `{"repository":"analyzer-conventional"}`

	c, _ := webhookRequest(body, map[string]string{"X-Webhook-Secret": "not-the-secret"})

	require.Error(t, handler.authenticateWebhook(c, []byte(body)))
}

// Plugin repositories that still send the shared secret keep working while they
// migrate to signatures.
func TestWebhookAcceptsLegacySharedSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSyncHandlerWithSecret(nil, testWebhookSecret)
	body := `{"repository":"analyzer-conventional"}`

	c, _ := webhookRequest(body, map[string]string{"X-Webhook-Secret": testWebhookSecret})

	require.NoError(t, handler.authenticateWebhook(c, []byte(body)))
}
