package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Get service configuration from environment
	port := getEnv("SERVICE_PORT", "8082")
	metricsPort := getEnv("METRICS_PORT", "8083")

	// Create main router
	router := gin.Default()

	// API routes
	v1 := router.Group("/api/v1")
	{
		v1.POST("/process", processIntent)
		v1.GET("/status/:id", getProcessingStatus)
	}

	// Create metrics router
	metricsRouter := gin.New()
	metricsRouter.GET("/health", healthCheck)
	metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start servers
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	metricsSrv := &http.Server{
		Addr:    ":" + metricsPort,
		Handler: metricsRouter,
	}

	// Start servers in goroutines
	go func() {
		logger.Info("Starting Intent Processor API", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start API server", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("Starting metrics server", zap.String("port", metricsPort))
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start metrics server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("API server forced to shutdown", zap.Error(err))
	}

	if err := metricsSrv.Shutdown(ctx); err != nil {
		logger.Error("Metrics server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func processIntent(c *gin.Context) {
	// Placeholder for intent processing logic
	c.JSON(http.StatusOK, gin.H{
		"id":      "intent-123",
		"status":  "processing",
		"message": "Intent received and queued for processing",
	})
}

func getProcessingStatus(c *gin.Context) {
	id := c.Param("id")
	// Placeholder for status retrieval logic
	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "completed",
		"result": map[string]interface{}{
			"processed_at": time.Now(),
			"success":      true,
		},
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "intent-processor",
		"version": getEnv("SERVICE_VERSION", "1.0.0"),
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}