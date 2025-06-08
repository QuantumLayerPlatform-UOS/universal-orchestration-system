package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// Logger middleware for request logging
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		fields := []zap.Field{
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status_code", statusCode),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		if errorMessage != "" {
			fields = append(fields, zap.String("error", errorMessage))
		}

		// Log based on status code
		switch {
		case statusCode >= 500:
			logger.Error("Server error", fields...)
		case statusCode >= 400:
			logger.Warn("Client error", fields...)
		case statusCode >= 300:
			logger.Info("Redirection", fields...)
		default:
			logger.Info("Request processed", fields...)
		}
	}
}

// Recovery middleware for panic recovery
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				// Return 500 error
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// CORS middleware for Cross-Origin Resource Sharing
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Auth middleware for authentication
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Check for API key
			apiKey := c.GetHeader("X-API-Key")
			if apiKey == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Missing authorization",
				})
				return
			}
			
			// Validate API key (simplified for now)
			// In production, this would check against a database
			if !isValidAPIKey(apiKey) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
				return
			}
			
			// Set user context from API key
			c.Set("user_id", "api_user")
			c.Set("auth_type", "api_key")
			c.Next()
			return
		}

		// Extract token from Bearer scheme
		tokenString := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization format",
			})
			return
		}

		// Validate JWT token (simplified for now)
		// In production, this would properly validate JWT
		userID, err := validateJWT(tokenString, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		// Set user context
		c.Set("user_id", userID)
		c.Set("auth_type", "jwt")
		c.Next()
	}
}

// RateLimit middleware for rate limiting
func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	// Simple in-memory rate limiter
	// In production, use Redis-based rate limiting
	clients := make(map[string][]time.Time)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		// Clean old entries
		if requests, exists := clients[clientIP]; exists {
			validRequests := []time.Time{}
			for _, t := range requests {
				if now.Sub(t) < time.Minute {
					validRequests = append(validRequests, t)
				}
			}
			clients[clientIP] = validRequests
		}
		
		// Check rate limit
		if len(clients[clientIP]) >= requestsPerMinute {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}
		
		// Add current request
		clients[clientIP] = append(clients[clientIP], now)
		c.Next()
	}
}

// RequestID middleware for request tracking
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// Timeout middleware for request timeout
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		
		c.Request = c.Request.WithContext(ctx)
		
		finished := make(chan struct{})
		go func() {
			c.Next()
			finished <- struct{}{}
		}()
		
		select {
		case <-finished:
			// Request completed within timeout
		case <-ctx.Done():
			// Timeout occurred
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
				"error": "Request timeout",
			})
		}
	}
}

// Tracing middleware for distributed tracing
func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}

// RequestSizeLimit middleware to limit request body size
func RequestSizeLimit(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		
		// Check if body is too large
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		
		if int64(len(body)) > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Request body too large",
			})
			return
		}
		
		c.Next()
	}
}

// Helper functions

func isValidAPIKey(apiKey string) bool {
	// Simplified validation
	// In production, check against database
	validKeys := []string{
		"test-api-key-123",
		"prod-api-key-456",
	}
	
	for _, key := range validKeys {
		if apiKey == key {
			return true
		}
	}
	return false
}

func validateJWT(token, secret string) (string, error) {
	// Simplified JWT validation
	// In production, use proper JWT library
	if token == "valid-test-token" {
		return "test_user", nil
	}
	return "", fmt.Errorf("invalid token")
}

func generateRequestID() string {
	// Simple request ID generation
	// In production, use UUID
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}