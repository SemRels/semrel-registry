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

	var pluginRepo repository.PluginRepository
	metricsRecorder := service.NewNoopMetricsRecorder()
	statsProvider := service.NewNoopRegistryStatsProvider()

	switch cfg.StorageBackend {
	case "file":
		log.Printf("using file storage backend at %s", cfg.StorageDir)
		repo, err := repository.NewFileRepository(cfg.StorageDir)
		if err != nil {
			log.Fatalf("file repository init failed: %v", err)
		}
		pluginRepo = repo

	default: // "postgres"
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

		deleted, normalized, err := db.CleanupSemrelDuplicates(context.Background())
		if err != nil {
			log.Printf("startup cleanup warning: %v", err)
		} else if deleted > 0 || normalized > 0 {
			log.Printf("startup cleanup: deleted %d duplicate rows, normalized %d rows", deleted, normalized)
		}

		pluginRepo = repository.NewPluginRepository(db)
		metricsRecorder = service.NewAsyncMetricsRecorder(db, service.MetricsConfig{
			BufferSize:    cfg.MetricsQueueSize,
			BatchSize:     cfg.MetricsBatchSize,
			FlushInterval: cfg.MetricsFlushInterval,
		})
		statsProvider = service.NewPostgresRegistryStatsProvider(db)
	}
	defer func() {
		if err := metricsRecorder.Close(context.Background()); err != nil {
			log.Printf("metrics shutdown warning: %v", err)
		}
	}()

	pluginService := service.NewPluginService(pluginRepo)

	// Auto-seed from plugins.json on first startup (when DB is empty).
	if err := seedPluginsIfEmpty(context.Background(), pluginService, os.Getenv("PLUGINS_JSON_PATH")); err != nil {
		log.Printf("seed warning: %v", err)
	}

	router := newRouter(pluginService, routerDependencies{
		metrics: metricsRecorder,
		stats:   statsProvider,
		rateLimCfg: middleware.RateLimitConfig{
			Enabled:    cfg.RateLimitEnabled,
			PublicRPM:  cfg.RateLimitPublicRPM,
			PluginsRPM: cfg.RateLimitPluginsRPM,
			AuthRPM:    cfg.RateLimitAuthRPM,
			TrustProxy: cfg.RateLimitTrustProxy,
		},
	})

	log.Printf("server listening on %s", cfg.Port)
	if err := router.Run(cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

type routerDependencies struct {
	metrics    service.MetricsRecorder
	stats      service.RegistryStatsProvider
	rateLimCfg middleware.RateLimitConfig
}

func newRouter(pluginService service.PluginManager, deps ...routerDependencies) *gin.Engine {
	router := gin.New()
	router.Use(handlers.ErrorHandler(), handlers.RequestLogger(), handlers.CORSMiddleware())

	metricsRecorder := service.NewNoopMetricsRecorder()
	statsProvider := service.NewNoopRegistryStatsProvider()
	var rlCfg middleware.RateLimitConfig
	if len(deps) > 0 {
		if deps[0].metrics != nil {
			metricsRecorder = deps[0].metrics
		}
		if deps[0].stats != nil {
			statsProvider = deps[0].stats
		}
		rlCfg = deps[0].rateLimCfg
	}

	// Rate limiting middleware instances (no-ops when Enabled=false).
	rlPublic := middleware.RateLimit(rlCfg, rlCfg.PublicRPM)
	rlPluginsJSON := middleware.RateLimit(rlCfg, rlCfg.PluginsRPM)
	rlAuth := middleware.RateLimit(rlCfg, rlCfg.AuthRPM)

	// GitHub OAuth routes (public) — rate limited.
	authHandler := handlers.NewAuthHandler()
	router.GET("/auth/github", rlAuth, authHandler.Redirect)
	router.GET("/auth/github/callback", rlAuth, authHandler.Callback)
	router.GET("/auth/callback", rlAuth, authHandler.Callback) // alias: GitHub App configured without /github
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
	requireAdmin := middleware.RequireAdmin(authHandler)

	// Public read endpoints — with OptionalAuth so admins can filter by status.
	optionalAuth := middleware.OptionalAuth(authHandler)
	pluginHandler := handlers.NewPluginHandler(pluginService, metricsRecorder)
	api.GET("/plugins", rlPublic, optionalAuth, pluginHandler.ListPlugins)
	api.GET("/plugins/:id", rlPublic, optionalAuth, pluginHandler.GetPlugin)
	api.GET("/plugins/:id/versions", rlPublic, pluginHandler.ListPluginVersions)
	api.GET("/plugins/:id/versions/:version/download", rlPublic, pluginHandler.DownloadPluginVersion)
	// Namespaced plugin lookup: GET /api/v1/plugins/@semrel/provider-github
	api.GET("/plugins/@:namespace/:name", rlPublic, optionalAuth, pluginHandler.GetPluginByNamespace)
	api.GET("/plugins/@:namespace/:name/versions", rlPublic, pluginHandler.ListPluginVersionsByNamespace)
	api.GET("/plugins/@:namespace/:name/versions/:version/download", rlPublic, pluginHandler.DownloadPluginVersionByNamespace)

	adminHandler := handlers.NewAdminHandler(pluginService, statsProvider)
	api.GET("/stats", requireAdmin, adminHandler.GetStats)

	// Plugin standards validation (public — no auth needed to check).
	api.POST("/plugins/validate", handlers.ValidatePlugin)

	syncHandler := handlers.NewSyncHandler(pluginService)

	// plugins.json — semrel registry metadata endpoint consumed by `semrel` CLI.
	// SEMREL_REGISTRY_URL=http://localhost:8080 and semrel fetches /plugins.json.
	router.GET("/plugins.json", rlPluginsJSON, syncHandler.PluginsJSON)

	// Webhook endpoint: receives repository_dispatch from plugin release workflows.
	// Protected by WEBHOOK_SECRET env var (optional but recommended in prod).
	api.POST("/webhooks/release", syncHandler.WebhookRelease)

	// Protected endpoints — any authenticated user.
	requireAuth := middleware.RequireAuth(authHandler)

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

	// JSON Schema serving — stable, versioned, publicly cached.
	schemaHandler := handlers.NewSchemaHandler()
	router.GET("/schemas/core/:version", schemaHandler.GetCoreSchema)
	router.GET("/schemas/plugins/:name/:version", schemaHandler.GetPluginSchema)
	router.GET("/schemas/plugins/@:namespace/:name/:version", schemaHandler.GetNamespacedPluginSchema)

	return router
}
