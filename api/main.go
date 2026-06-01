package main

import (
	"context"
	"log"
	"os"

	"github.com/SemRels/semrel-registry/api/config"
	"github.com/SemRels/semrel-registry/api/database"
	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/SemRels/semrel-registry/api/middleware"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	if cfg.Environment == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if err := db.RunMigrations(cfg.MigrateDir); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	pluginRepo := repository.NewPluginRepository(db)
	pluginService := service.NewPluginService(pluginRepo)

	// Auto-seed from plugins.json on first startup (when DB is empty).
	if err := seedPluginsIfEmpty(context.Background(), pluginService, os.Getenv("PLUGINS_JSON_PATH")); err != nil {
		log.Printf("seed warning: %v", err)
	}

	router := newRouter(pluginService)

	log.Printf("server listening on %s", cfg.Port)
	if err := router.Run(cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func newRouter(pluginService service.PluginManager) *gin.Engine {
	router := gin.New()
	router.Use(handlers.ErrorHandler(), handlers.RequestLogger(), handlers.CORSMiddleware())

	// GitHub OAuth routes (public).
	authHandler := handlers.NewAuthHandler()
	router.GET("/auth/github", authHandler.Redirect)
	router.GET("/auth/github/callback", authHandler.Callback)
	router.GET("/auth/callback", authHandler.Callback)  // alias: GitHub App configured without /github
	router.GET("/oauth/callback", authHandler.Callback) // alias for GitHub App callback URL
	router.GET("/auth/config", authHandler.Config)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"name":    "semrel-registry",
			"version": "1",
			"docs":    "https://semrel.io",
			"health":  "/health",
			"plugins": "/api/v1/plugins",
		})
	})
	router.GET("/health", handlers.Health())

	api := router.Group("/api/v1")

	// Public read endpoints — with OptionalAuth so admins can filter by status.
	optionalAuth := middleware.OptionalAuth(authHandler)
	pluginHandler := handlers.NewPluginHandler(pluginService)
	api.GET("/plugins", optionalAuth, pluginHandler.ListPlugins)
	api.GET("/plugins/:id", optionalAuth, pluginHandler.GetPlugin)
	api.GET("/plugins/:id/versions", pluginHandler.ListPluginVersions)

	adminHandler := handlers.NewAdminHandler(pluginService)
	api.GET("/stats", adminHandler.GetStats)

	// Plugin standards validation (public — no auth needed to check).
	api.POST("/plugins/validate", handlers.ValidatePlugin)

	syncHandler := handlers.NewSyncHandler(pluginService)

	// plugins.json — semrel registry metadata endpoint consumed by `semrel` CLI.
	// SEMREL_REGISTRY_URL=http://localhost:8080 and semrel fetches /plugins.json.
	router.GET("/plugins.json", syncHandler.PluginsJSON)

	// Webhook endpoint: receives repository_dispatch from plugin release workflows.
	// Protected by WEBHOOK_SECRET env var (optional but recommended in prod).
	api.POST("/webhooks/release", syncHandler.WebhookRelease)

	// Protected endpoints — any authenticated user.
	requireAuth  := middleware.RequireAuth(authHandler)
	requireAdmin := middleware.RequireAdmin(authHandler)

	authRoutes := api.Group("")
	authRoutes.Use(requireAuth)
	authRoutes.GET("/auth/me", authHandler.Me)
	// Community plugin submission (creates with status=pending for review).
	authRoutes.POST("/plugins/submit", pluginHandler.SubmitPlugin)
	// Plugin writes: any authenticated user, but non-admins may only touch their own plugins.
	authRoutes.POST("/plugins", pluginHandler.CreatePlugin)
	authRoutes.PUT("/plugins/:id", pluginHandler.UpdatePlugin)
	authRoutes.DELETE("/plugins/:id", pluginHandler.DeletePlugin)
	authRoutes.POST("/plugins/:id/versions", pluginHandler.CreatePluginVersion)

	// Admin-only endpoints.
	adminRoutes := api.Group("")
	adminRoutes.Use(requireAdmin)
	adminRoutes.POST("/admin/sync", adminHandler.SyncPlugins)
	adminRoutes.POST("/admin/sync-file", adminHandler.SyncFromFile)
	adminRoutes.GET("/admin/status", adminHandler.Status)
	adminRoutes.POST("/admin/sync-versions", syncHandler.SyncVersions)
	adminRoutes.POST("/admin/sync-github-org", syncHandler.SyncGitHubOrg)
	adminRoutes.PUT("/admin/plugins/:id/approve", pluginHandler.ApprovePlugin)
	adminRoutes.PUT("/admin/plugins/:id/reject", pluginHandler.RejectPlugin)
	adminRoutes.POST("/admin/plugins/:id/revalidate", pluginHandler.RevalidatePlugin)

	return router
}
