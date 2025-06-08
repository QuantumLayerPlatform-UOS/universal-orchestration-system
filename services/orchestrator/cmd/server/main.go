package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quantumlayer/uos/services/orchestrator/internal/config"
	"github.com/quantumlayer/uos/services/orchestrator/internal/database"
	"github.com/quantumlayer/uos/services/orchestrator/internal/handlers"
	"github.com/quantumlayer/uos/services/orchestrator/internal/middleware"
	"github.com/quantumlayer/uos/services/orchestrator/internal/services"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize services
	orchestratorService := services.NewOrchestratorService(db, logger)
	workflowService := services.NewWorkflowService(cfg.Temporal.HostPort, logger)

	// Initialize handlers
	handlers := handlers.NewHandlers(orchestratorService, workflowService, logger)

	// Setup router
	router := setupRouter(handlers, logger)

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Orchestrator service started", zap.String("port", cfg.Server.Port))

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}

func setupRouter(h *handlers.Handlers, logger *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Projects
		projects := v1.Group("/projects")
		{
			projects.POST("", h.CreateProject)
			projects.GET("/:id", h.GetProject)
			projects.GET("", h.ListProjects)
			projects.PUT("/:id", h.UpdateProject)
			projects.DELETE("/:id", h.DeleteProject)
		}

		// Workflows
		workflows := v1.Group("/workflows")
		{
			workflows.POST("", h.StartWorkflow)
			workflows.GET("/:id", h.GetWorkflow)
			workflows.GET("", h.ListWorkflows)
			workflows.POST("/:id/cancel", h.CancelWorkflow)
		}

		// Agents
		agents := v1.Group("/agents")
		{
			agents.GET("", h.ListAgents)
			agents.GET("/:id", h.GetAgent)
			agents.POST("/:id/restart", h.RestartAgent)
		}
	}

	return router
}
