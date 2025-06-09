package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.uber.org/zap"

	"orchestrator/internal/api"
	"orchestrator/internal/config"
	"orchestrator/internal/database"
	"orchestrator/internal/middleware"
	"orchestrator/internal/services"
	"orchestrator/internal/temporal"
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

	// Initialize telemetry
	if cfg.Telemetry.Enabled {
		shutdown, err := initTelemetry(cfg, logger)
		if err != nil {
			logger.Error("Failed to initialize telemetry", zap.Error(err))
		} else {
			defer shutdown(context.Background())
		}
	}

	// Initialize database
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		MaxRetries:   cfg.Redis.MaxRetries,
		MinIdleConns: cfg.Redis.MinIdleConns,
		MaxIdleConns: cfg.Redis.MaxActiveConns,
		PoolTimeout:  time.Duration(cfg.Redis.PoolTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize clients
	intentClient, err := services.NewIntentClient(&cfg.IntentAPI, logger)
	if err != nil {
		logger.Fatal("Failed to create intent client", zap.Error(err))
	}
	defer intentClient.Close()

	agentClient, err := services.NewAgentClient(&cfg.AgentManager, logger)
	if err != nil {
		logger.Fatal("Failed to create agent client", zap.Error(err))
	}
	defer agentClient.Close()

	// Initialize Temporal worker
	temporalWorker, err := temporal.NewWorker(&cfg.Temporal, db, logger, intentClient, agentClient)
	if err != nil {
		logger.Fatal("Failed to create Temporal worker", zap.Error(err))
	}

	// Start Temporal worker in background
	go func() {
		if err := temporalWorker.Start(); err != nil {
			logger.Fatal("Failed to start Temporal worker", zap.Error(err))
		}
	}()
	defer temporalWorker.Stop()

	// Initialize services
	projectService := services.NewProjectService(db, logger)
	
	// Create workflow engine with proper configuration
	workflowConfig := &services.WorkflowConfig{
		TaskQueue:               cfg.Temporal.TaskQueue,
		MaxConcurrentWorkflows:  cfg.Temporal.MaxConcurrentWorkflows,
		MaxConcurrentActivities: cfg.Temporal.MaxConcurrentActivities,
		WorkflowTimeout:         30 * time.Minute,
		ActivityTimeout:         5 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
		EnableMetrics: cfg.Temporal.EnableMetrics,
		EnableTracing: cfg.Telemetry.EnableDistributedTracing,
	}
	
	workflowEngine := services.NewWorkflowEngine(
		db,
		redisClient,
		temporalWorker.GetClient(),
		logger,
		intentClient,
		agentClient,
		workflowConfig,
	)

	// Initialize workflow monitor
	workflowMonitor := services.NewWorkflowMonitor(
		db,
		temporalWorker.GetClient(),
		logger,
		redisClient,
		5*time.Second, // Check every 5 seconds
	)
	workflowMonitor.Start()
	defer workflowMonitor.Stop()

	// Initialize handlers
	handlers := api.NewHandlers(workflowEngine, projectService, agentClient, logger)

	// Setup routers
	router := setupRouter(handlers, cfg, logger)
	
	// Start metrics server if enabled
	if cfg.Server.EnableMetrics {
		go startMetricsServer(cfg.Server.MetricsPort, logger)
	}

	// Start main server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting Orchestrator service",
			zap.String("host", cfg.Server.Host),
			zap.String("port", cfg.Server.Port),
			zap.String("environment", cfg.Telemetry.Environment),
		)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(), 
		time.Duration(cfg.Server.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func setupRouter(h *api.Handlers, cfg *config.Config, logger *zap.Logger) *gin.Engine {
	// Set Gin mode
	if cfg.Telemetry.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.RequestSizeLimit(cfg.Server.MaxRequestSize))
	router.Use(middleware.Timeout(time.Duration(cfg.Server.WriteTimeout) * time.Second))

	// Tracing middleware if enabled
	if cfg.Telemetry.EnableDistributedTracing {
		router.Use(middleware.Tracing(cfg.Telemetry.ServiceName))
	}

	// Health check (no auth required)
	router.GET("/health", h.HealthCheck)
	router.GET("/ready", func(c *gin.Context) {
		// Ready check - simplified for now
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	
	// Apply auth middleware if enabled
	if cfg.Auth.Enabled {
		v1.Use(middleware.Auth(cfg.Auth.JWTSecret))
	}
	
	// Apply rate limiting
	v1.Use(middleware.RateLimit(1000)) // 1000 requests per minute

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
		workflows.GET("/:id/metrics", h.GetWorkflowMetrics)
	}

	// Agents
	agents := v1.Group("/agents")
	{
		agents.GET("", h.ListAgents)
		agents.GET("/:id", h.GetAgent)
		agents.POST("/:id/restart", h.RestartAgent)
	}

	return router
}

func initTelemetry(cfg *config.Config, logger *zap.Logger) (func(context.Context) error, error) {
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.Telemetry.ServiceName),
			semconv.ServiceVersion(cfg.Telemetry.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Telemetry.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create Jaeger exporter
	exp, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(cfg.Telemetry.Jaeger.CollectorEndpoint),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.Telemetry.SamplingRate)),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	logger.Info("Telemetry initialized",
		zap.String("service", cfg.Telemetry.ServiceName),
		zap.String("environment", cfg.Telemetry.Environment),
	)

	// Return shutdown function
	return tp.Shutdown, nil
}

func startMetricsServer(port string, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"orchestrator","version":"1.0.0"}`)
	})
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	logger.Info("Starting metrics server", zap.String("port", port))
	
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Metrics server error", zap.Error(err))
	}
}