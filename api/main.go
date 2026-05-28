package main

import (
	"log"

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
	router.GET("/auth/config", authHandler.Config)

	router.GET("/health", handlers.Health())

	api := router.Group("/api/v1")

	// Public read endpoints.
	pluginHandler := handlers.NewPluginHandler(pluginService)
	api.GET("/plugins", pluginHandler.ListPlugins)
	api.GET("/plugins/:id", pluginHandler.GetPlugin)
	api.GET("/plugins/:id/versions", pluginHandler.ListPluginVersions)

	adminHandler := handlers.NewAdminHandler(pluginService)
	api.GET("/stats", adminHandler.GetStats)

	// Plugin standards validation (public — no auth needed to check).
	api.POST("/plugins/validate", handlers.ValidatePlugin)

	syncHandler := handlers.NewSyncHandler(pluginService)

	// Webhook endpoint: receives repository_dispatch from plugin release workflows.
	// Protected by WEBHOOK_SECRET env var (optional but recommended in prod).
	api.POST("/webhooks/release", syncHandler.WebhookRelease)

	// Protected endpoints — accept GitHub JWT or legacy ADMIN_TOKEN.
	requireAdmin := middleware.RequireAdmin(authHandler)
	protected := api.Group("")
	protected.Use(requireAdmin)
	protected.GET("/auth/me", authHandler.Me)
	protected.POST("/plugins", pluginHandler.CreatePlugin)
	protected.PUT("/plugins/:id", pluginHandler.UpdatePlugin)
	protected.DELETE("/plugins/:id", pluginHandler.DeletePlugin)
	protected.POST("/plugins/:id/versions", pluginHandler.CreatePluginVersion)
	protected.POST("/admin/sync", adminHandler.SyncPlugins)
	protected.POST("/admin/sync-file", adminHandler.SyncFromFile)
	protected.GET("/admin/status", adminHandler.Status)
	protected.POST("/admin/sync-versions", syncHandler.SyncVersions)

	return router
}
