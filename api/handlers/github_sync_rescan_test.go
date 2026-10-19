package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

func newRescanRouter(t *testing.T, login string, isAdmin bool) (*gin.Engine, string) {
	t.Helper()
	repo, err := repository.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := service.NewPluginService(repo)
	created, err := manager.CreatePlugin(context.Background(), models.Plugin{
		Name: "npm", Category: "updater", Author: "alice",
		Repository: "https://github.com/SemRels/updater-npm",
		Status:     models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("login", login)
		c.Set("isAdmin", isAdmin)
	})
	router.POST("/plugins/:id/sync-versions", NewSyncHandler(manager).RescanPluginVersions)
	return router, strconv.FormatInt(int64(created.ID), 10)
}

func TestRescanPluginVersionsRejectsOtherUsers(t *testing.T) {
	router, ref := newRescanRouter(t, "mallory", false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/plugins/"+ref+"/sync-versions", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d, body=%s", response.Code, http.StatusForbidden, response.Body)
	}
}

func TestRescanPluginVersionsUnknownPlugin(t *testing.T) {
	router, _ := newRescanRouter(t, "alice", false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/plugins/does-not-exist/sync-versions", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d, body=%s", response.Code, http.StatusNotFound, response.Body)
	}
}
