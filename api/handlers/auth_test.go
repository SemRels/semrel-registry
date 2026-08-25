package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAccountManager struct {
	request models.AccountDeletionRequest
	actor   models.DeleteActor
	result  models.AccountDeletionResult
	err     error
}

func (s *stubAccountManager) ListPlugins(context.Context, service.ListPluginsParams) (service.PluginListResult, error) {
	return service.PluginListResult{}, nil
}
func (s *stubAccountManager) GetPlugin(context.Context, string) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) ListVersions(context.Context, string, int, int) ([]models.PluginVersion, error) {
	return nil, nil
}
func (s *stubAccountManager) CreatePlugin(context.Context, models.Plugin) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) SubmitPlugin(context.Context, models.Plugin) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) UpdatePlugin(context.Context, string, models.PluginPatch) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) DeletePlugin(context.Context, string) error { return nil }
func (s *stubAccountManager) DeletePluginWithRequest(context.Context, string, models.PluginDeletionRequest, models.DeleteActor) error {
	return nil
}
func (s *stubAccountManager) CreateVersion(context.Context, string, models.PluginVersion) (models.PluginVersion, error) {
	return models.PluginVersion{}, nil
}
func (s *stubAccountManager) DeleteVersion(context.Context, int64, int64) error { return nil }
func (s *stubAccountManager) DeleteVersionWithRequest(context.Context, string, int64, models.VersionDeletionRequest, models.DeleteActor) error {
	return nil
}
func (s *stubAccountManager) DeleteAccount(_ context.Context, request models.AccountDeletionRequest, actor models.DeleteActor) (models.AccountDeletionResult, error) {
	s.request = request
	s.actor = actor
	return s.result, s.err
}
func (s *stubAccountManager) ApprovePlugin(context.Context, string) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) RejectPlugin(context.Context, string) (models.Plugin, error) {
	return models.Plugin{}, nil
}
func (s *stubAccountManager) UpdateValidationChecks(context.Context, int64, []byte) error { return nil }

func TestDeleteAccountRequiresReauthAndDelegates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := &stubAccountManager{result: models.AccountDeletionResult{PluginsDeleted: 2, VersionsDeleted: 5}}
	handler := NewAuthHandler(manager)

	router := gin.New()
	router.DELETE("/auth/me", func(c *gin.Context) {
		c.Set("login", "alice")
		c.Set("isAdmin", false)
		c.Next()
	}, handler.DeleteAccount)

	req := httptest.NewRequest(http.MethodDelete, "/auth/me", mustJSONReader(t, map[string]any{
		"confirmation":       "DELETE alice",
		"reauthToken":        "secret",
		"deleteOwnedPlugins": true,
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "alice", manager.actor.Login)
	assert.True(t, manager.request.DeleteOwnedPlugins)

	var payload struct {
		Data models.AccountDeletionResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, 2, payload.Data.PluginsDeleted)
	assert.Equal(t, 5, payload.Data.VersionsDeleted)
}

func TestDeleteAccountRejectsWrongReauthToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&stubAccountManager{})

	router := gin.New()
	router.DELETE("/auth/me", func(c *gin.Context) {
		c.Set("login", "alice")
		c.Next()
	}, handler.DeleteAccount)

	req := httptest.NewRequest(http.MethodDelete, "/auth/me", mustJSONReader(t, map[string]any{
		"confirmation":       "DELETE alice",
		"reauthToken":        "wrong",
		"deleteOwnedPlugins": true,
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func mustJSONReader(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return bytes.NewReader(data)
}
