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
	port := getEnv("SERVICE_PORT", "8084")
	metricsPort := getEnv("METRICS_PORT", "8085")

	// Create main router
	router := gin.Default()

	// API routes
	v1 := router.Group("/api/v1")
	{
		v1.GET("/agents", listAgents)
		v1.POST("/agents", createAgent)
		v1.GET("/agents/:id", getAgent)
		v1.PUT("/agents/:id", updateAgent)
		v1.DELETE("/agents/:id", deleteAgent)
		v1.POST("/agents/:id/execute", executeAgentTask)
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
		logger.Info("Starting Agent Manager API", zap.String("port", port))
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

func listAgents(c *gin.Context) {
	// Placeholder for agent listing logic
	c.JSON(http.StatusOK, gin.H{
		"agents": []map[string]interface{}{
			{
				"id":     "agent-1",
				"name":   "Infrastructure Agent",
				"type":   "terraform",
				"status": "active",
			},
			{
				"id":     "agent-2",
				"name":   "Monitoring Agent",
				"type":   "prometheus",
				"status": "active",
			},
		},
	})
}

func createAgent(c *gin.Context) {
	// Placeholder for agent creation logic
	c.JSON(http.StatusCreated, gin.H{
		"id":         "agent-3",
		"name":       "New Agent",
		"created_at": time.Now(),
	})
}

func getAgent(c *gin.Context) {
	id := c.Param("id")
	// Placeholder for agent retrieval logic
	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"name":   "Infrastructure Agent",
		"type":   "terraform",
		"status": "active",
		"config": map[string]interface{}{
			"provider": "aws",
			"region":   "us-east-1",
		},
	})
}

func updateAgent(c *gin.Context) {
	id := c.Param("id")
	// Placeholder for agent update logic
	c.JSON(http.StatusOK, gin.H{
		"id":         id,
		"updated_at": time.Now(),
		"message":    "Agent updated successfully",
	})
}

func deleteAgent(c *gin.Context) {
	id := c.Param("id")
	// Placeholder for agent deletion logic
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Agent deleted successfully",
	})
}

func executeAgentTask(c *gin.Context) {
	id := c.Param("id")
	// Placeholder for task execution logic
	c.JSON(http.StatusAccepted, gin.H{
		"agent_id":    id,
		"task_id":     "task-123",
		"status":      "queued",
		"message":     "Task queued for execution",
		"started_at":  time.Now(),
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "agent-manager",
		"version": getEnv("SERVICE_VERSION", "1.0.0"),
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}