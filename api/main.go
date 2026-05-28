package main

import (
	"log"

	"github.com/SemRels/semrel-registry/api/config"
	"github.com/SemRels/semrel-registry/api/database"
	"github.com/SemRels/semrel-registry/api/handlers"
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

	router.GET("/health", handlers.Health())

	api := router.Group("/api/v1")
	pluginHandler := handlers.NewPluginHandler(pluginService)
	api.GET("/plugins", pluginHandler.ListPlugins)
	api.GET("/plugins/:id", pluginHandler.GetPlugin)
	api.GET("/plugins/:id/versions", pluginHandler.ListPluginVersions)

	admin := handlers.NewAdminHandler(pluginService)
	api.GET("/stats", admin.GetStats)

	adminRoutes := api.Group("")
	adminRoutes.Use(handlers.RequireAdminToken())
	adminRoutes.POST("/plugins", pluginHandler.CreatePlugin)
	adminRoutes.PUT("/plugins/:id", pluginHandler.UpdatePlugin)
	adminRoutes.DELETE("/plugins/:id", pluginHandler.DeletePlugin)
	adminRoutes.POST("/plugins/:id/versions", pluginHandler.CreatePluginVersion)
	adminRoutes.POST("/admin/sync", admin.SyncPlugins)
	adminRoutes.POST("/admin/sync-file", admin.SyncFromFile)
	adminRoutes.GET("/admin/status", admin.Status)

	return router
}
