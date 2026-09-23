package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SemRels/semrel-registry/api/config"
	"github.com/SemRels/semrel-registry/api/database"
	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/SemRels/semrel-registry/api/middleware"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Fail fast on an insecure configuration rather than starting a server that
	// looks healthy while accepting forged admin tokens or unauthenticated
	// webhooks. In development the same problems are surfaced as warnings.
	warnings, err := cfg.Validate()
	for _, warning := range warnings {
		log.Printf("config warning: %s", warning)
	}
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	service.SetAllowedArtifactHosts(cfg.AllowedDownloadHosts)

	var pluginRepo repository.PluginRepository
	var webhookRepo repository.WebhookRepository
	var postgresDB *database.Database
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
		metricsRecorder = service.NewFileMetricsRecorder(repo, cfg.MetricsFlushInterval)
		webhooks, err := repository.NewFileWebhookRepository(cfg.StorageDir)
		if err != nil {
			log.Fatalf("webhook repository init failed: %v", err)
		}
		webhookRepo = webhooks

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
		postgresDB = db

		deleted, normalized, err := db.CleanupSemrelDuplicates(context.Background())
		if err != nil {
			log.Printf("startup cleanup warning: %v", err)
		} else if deleted > 0 || normalized > 0 {
			log.Printf("startup cleanup: deleted %d duplicate rows, normalized %d rows", deleted, normalized)
		}

		pluginRepo = repository.NewPluginRepository(db)
		webhookRepo = repository.NewWebhookRepository(db)
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

	// Review outcomes are delivered only when a relay is configured. Without
	// one the decision is still recorded and shown to the author in the UI,
	// and the attempt is logged so the path stays exercised in development.
	smtpCfg := service.SMTPConfig{
		Host:       cfg.SMTPHost,
		Port:       cfg.SMTPPort,
		Username:   cfg.SMTPUsername,
		Password:   cfg.SMTPPassword,
		From:       cfg.SMTPFrom,
		AppBaseURL: cfg.FrontendURL,
	}
	var notifier service.ReviewNotifier = service.LoggingNotifier{}
	if smtpCfg.Configured() {
		notifier = service.NewSMTPNotifier(smtpCfg)
		log.Printf("review notifications will be sent via %s", cfg.SMTPHost)
	}

	pluginService := service.NewPluginServiceWithNotifier(pluginRepo, notifier)

	var pool *pgxpool.Pool
	if postgresDB != nil {
		pool = postgresDB.Pool()
	}
	source := resolveCatalogSource(os.Getenv("PLUGINS_JSON_PATH"), cfg.Environment)
	if err := seedStartupCatalog(context.Background(), pluginRepo, pool, source); err != nil {
		log.Fatalf("startup seed failed: %v", err)
	}

	router := newRouter(pluginService, routerDependencies{
		metrics:  metricsRecorder,
		stats:    statsProvider,
		webhooks: webhookRepo,
		rateLimCfg: middleware.RateLimitConfig{
			Enabled:    cfg.RateLimitEnabled,
			PublicRPM:  cfg.RateLimitPublicRPM,
			PluginsRPM: cfg.RateLimitPluginsRPM,
			AuthRPM:    cfg.RateLimitAuthRPM,
			WriteRPM:   cfg.RateLimitWriteRPM,
			TrustProxy: cfg.RateLimitTrustProxy,
		},
		cfg: cfg,
	})

	// Only peers inside the configured proxy ranges may set X-Forwarded-For.
	// Without this, any client can choose the IP every per-IP control keys on.
	trustedProxies := cfg.TrustedProxies
	if !cfg.RateLimitTrustProxy {
		trustedProxies = nil
	}
	if proxyErr := router.SetTrustedProxies(trustedProxies); proxyErr != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", proxyErr)
	}

	server := &http.Server{
		Addr:    cfg.Port,
		Handler: router,
		// Bound every phase of a request: an idle or slow-loris connection must
		// not be able to hold a server slot open indefinitely.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("server listening on %s", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", serveErr)
		}
	}()

	// Drain in-flight requests on SIGTERM so a deploy does not cut off a write
	// midway through.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("shutdown signal received; draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("graceful shutdown failed: %v", shutdownErr)
	}
}

type routerDependencies struct {
	metrics    service.MetricsRecorder
	stats      service.RegistryStatsProvider
	webhooks   repository.WebhookRepository
	rateLimCfg middleware.RateLimitConfig
	cfg        *config.Config
}

func newRouter(pluginService service.PluginManager, deps ...routerDependencies) *gin.Engine {
	metricsRecorder := service.NewNoopMetricsRecorder()
	statsProvider := service.NewNoopRegistryStatsProvider()
	var webhookRepo repository.WebhookRepository
	var rlCfg middleware.RateLimitConfig
	cfg := &config.Config{}
	if len(deps) > 0 {
		if deps[0].metrics != nil {
			metricsRecorder = deps[0].metrics
		}
		if deps[0].stats != nil {
			statsProvider = deps[0].stats
		}
		webhookRepo = deps[0].webhooks
		rlCfg = deps[0].rateLimCfg
		if deps[0].cfg != nil {
			cfg = deps[0].cfg
		}
	}

	router := gin.New()
	router.Use(
		handlers.ErrorHandler(),
		handlers.RequestLogger(),
		handlers.SecurityHeaders(),
		handlers.CORS(handlers.CORSOptions{AllowedOrigins: cfg.AllowedOrigins}),
		handlers.LimitRequestBody(cfg.MaxRequestKiB),
		middleware.VerifyOrigin(cfg.AllowedOrigins),
	)

	// Rate limiting middleware instances (no-ops when Enabled=false).
	rlPublic := middleware.RateLimit(rlCfg, rlCfg.PublicRPM)
	rlPluginsJSON := middleware.RateLimit(rlCfg, rlCfg.PluginsRPM)
	rlAuth := middleware.RateLimit(rlCfg, rlCfg.AuthRPM)
	rlWrite := middleware.RateLimit(rlCfg, rlCfg.WriteRPM)

	// GitHub OAuth routes (public) — rate limited.
	authHandler := handlers.NewAuthHandlerWithOptions(handlers.AuthOptions{
		JWTSecret:    cfg.JWTSecret,
		AdminToken:   cfg.AdminToken,
		FrontendURL:  cfg.FrontendURL,
		SessionTTL:   cfg.SessionTTL,
		CookieName:   cfg.CookieName,
		CookieDomain: cfg.CookieDomain,
		CookieSecure: cfg.CookieSecure,
	}, pluginService)
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
	pluginHandler := handlers.NewPluginHandler(pluginService, metricsRecorder).WithWebhooks(webhookRepo)
	api.GET("/plugins", rlPublic, optionalAuth, pluginHandler.ListPlugins)
	api.GET("/plugins/:id", rlPublic, optionalAuth, pluginHandler.GetPlugin)
	api.GET("/plugins/:id/versions", rlPublic, pluginHandler.ListPluginVersions)
	// Proxied from GitHub and cached: the browser has no API token, and a direct
	// fetch would disclose every visitor's address to GitHub.
	api.GET("/plugins/:id/readme", rlWrite, pluginHandler.PluginReadme)
	api.GET("/plugins/:id/versions/:version/download", rlPublic, pluginHandler.DownloadPluginVersion)
	api.POST("/plugins/:id/versions/:version/downloads", rlPublic, pluginHandler.TrackDownload)
	// Namespaced plugin lookup: GET /api/v1/plugins/@semrel/provider-github
	api.GET("/plugins/@:namespace/:name", rlPublic, optionalAuth, pluginHandler.GetPluginByNamespace)
	api.GET("/plugins/@:namespace/:name/versions", rlPublic, pluginHandler.ListPluginVersionsByNamespace)
	api.GET("/plugins/@:namespace/:name/versions/:version/download", rlPublic, pluginHandler.DownloadPluginVersionByNamespace)
	api.POST("/plugins/@:namespace/:name/versions/:version/downloads", rlPublic, pluginHandler.TrackDownloadByNamespace)
	// Checks installed plugin@version pairs against known security advisories
	// — what `semrel plugin audit` calls. Read-only, so no authentication.
	api.POST("/audit", rlPublic, pluginHandler.AuditPlugins)

	adminHandler := handlers.NewAdminHandler(pluginService, statsProvider)
	api.GET("/stats", requireAdmin, adminHandler.GetStats)

	// Plugin standards validation. Each call fans out into several GitHub API
	// requests against the registry's shared token budget, so it is throttled
	// harder than an ordinary read even though it needs no authentication.
	api.POST("/plugins/validate", rlWrite, handlers.ValidatePlugin)

	syncHandler := handlers.NewSyncHandlerWithSecret(pluginService, cfg.WebhookSecret).WithWebhooks(webhookRepo)

	// Sitemap for SEO — lists all active plugin pages.
	sitemapHandler := handlers.NewSitemapHandler(pluginService)
	router.GET("/sitemap.xml", sitemapHandler.Sitemap)

	// Atom feed of recent releases, so consumers can follow the registry
	// without polling and diffing the catalogue themselves.
	feedHandler := handlers.NewFeedHandler(pluginService)
	router.GET("/feed.atom", rlPublic, feedHandler.Releases)

	// Machine-readable API description, plus a rendered reference.
	router.GET("/openapi.json", handlers.OpenAPISpec())
	router.GET("/docs", handlers.APIDocs())

	// plugins.json — semrel registry metadata endpoint consumed by `semrel` CLI.
	// SEMREL_REGISTRY_URL=http://localhost:8080 and semrel fetches /plugins.json.
	router.GET("/plugins.json", rlPluginsJSON, syncHandler.PluginsJSON)

	// Webhook endpoint: receives repository_dispatch from plugin release workflows.
	// Authenticated with WEBHOOK_SECRET (HMAC signature); required in production.
	api.POST("/webhooks/release", rlWrite, syncHandler.WebhookRelease)

	// Protected endpoints — any authenticated user.
	requireAuth := middleware.RequireAuth(authHandler)

	authRoutes := api.Group("")
	authRoutes.Use(requireAuth, rlWrite)
	authRoutes.GET("/auth/me", authHandler.Me)
	authRoutes.POST("/auth/logout", authHandler.Logout)
	authRoutes.DELETE("/auth/me", authHandler.DeleteAccount)
	// Community plugin submission (creates with status=pending for review).
	authRoutes.POST("/plugins/submit", pluginHandler.SubmitPlugin)
	// Checked before submitting, so a failed ownership claim surfaces while the
	// submitter can still act on it.
	authRoutes.POST("/plugins/verify-ownership", handlers.VerifyOwnership)
	// Plugin writes: any authenticated user, but non-admins may only touch their own plugins.
	authRoutes.POST("/plugins", pluginHandler.CreatePlugin)
	authRoutes.PUT("/plugins/:id", pluginHandler.UpdatePlugin)
	authRoutes.DELETE("/plugins/:id", pluginHandler.DeletePlugin)
	authRoutes.POST("/plugins/:id/versions", pluginHandler.CreatePluginVersion)
	authRoutes.DELETE("/plugins/:id/versions/:versionId", pluginHandler.DeletePluginVersion)
	// Yank retracts a release without breaking builds that already pin it.
	authRoutes.PUT("/plugins/:id/versions/:versionId/yank", pluginHandler.YankVersion)
	authRoutes.DELETE("/plugins/:id/versions/:versionId/yank", pluginHandler.UnyankVersion)
	// Uses :version (not :versionId) because gin shares one wildcard name per
	// path slot across all methods, and the public downloads-counter route
	// already registered ":version" as a POST at this same position.
	authRoutes.POST("/plugins/:id/versions/:version/reverify-provenance", pluginHandler.ReverifyProvenance)
	authRoutes.POST("/plugins/:id/advisories/refresh", pluginHandler.RefreshSecurityAdvisories)

	// Consumer webhook subscriptions: any authenticated account, not just a
	// plugin's own publisher, can ask to be notified about a plugin's events.
	webhookHandler := handlers.NewWebhookHandler(webhookRepo, pluginService)
	authRoutes.POST("/webhooks/subscriptions", webhookHandler.CreateSubscription)
	authRoutes.GET("/webhooks/subscriptions", webhookHandler.ListSubscriptions)
	authRoutes.DELETE("/webhooks/subscriptions/:id", webhookHandler.DeleteSubscription)

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
	adminRoutes.POST("/admin/plugins/revalidate-all", pluginHandler.RevalidateAllPlugins)

	// JSON Schema serving — stable, versioned, publicly cached.
	schemaHandler := handlers.NewSchemaHandler()
	router.GET("/schemas/core/:version", schemaHandler.GetCoreSchema)
	router.GET("/schemas/plugins/:name/:version", schemaHandler.GetPluginSchema)
	router.GET("/schemas/plugins/@:namespace/:name/:version", schemaHandler.GetNamespacedPluginSchema)

	return router
}
