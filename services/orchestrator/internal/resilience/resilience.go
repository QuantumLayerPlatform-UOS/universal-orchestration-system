package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// CircuitBreaker wraps the gobreaker circuit breaker
type CircuitBreaker struct {
	cb     *gobreaker.CircuitBreaker
	logger *zap.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, logger *zap.Logger) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    time.Minute,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	}

	return &CircuitBreaker{
		cb:     gobreaker.NewCircuitBreaker(settings),
		logger: logger,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return cb.cb.Execute(fn)
}

// ExecuteContext runs a function with circuit breaker protection and context
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	resultCh := make(chan interface{})
	errCh := make(chan error)

	go func() {
		result, err := cb.cb.Execute(fn)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		return result, nil
	}
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	limiter *rate.Limiter
	logger  *zap.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps int, burst int, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
		logger:  logger,
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// Wait waits for permission to proceed
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

// RetryConfig contains retry configuration
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(ctx context.Context, config RetryConfig, logger *zap.Logger, fn func() error) error {
	var err error
	backoff := config.InitialBackoff

	for i := 0; i <= config.MaxRetries; i++ {
		if i > 0 {
			logger.Debug("Retrying operation",
				zap.Int("attempt", i),
				zap.Duration("backoff", backoff),
				zap.Error(err))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}

			// Calculate next backoff
			backoff = time.Duration(float64(backoff) * config.Multiplier)
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}

		err = fn()
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, err)
}

// HealthChecker manages health checks
type HealthChecker struct {
	checks map[string]HealthCheck
	mu     sync.RWMutex
	logger *zap.Logger
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name      string
	CheckFunc func(ctx context.Context) error
	Timeout   time.Duration
}

// HealthStatus represents the status of a health check
type HealthStatus struct {
	Name    string        `json:"name"`
	Status  string        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Latency time.Duration `json:"latency"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheck),
		logger: logger,
	}
}

// Register registers a new health check
func (hc *HealthChecker) Register(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[check.Name] = check
}

// CheckAll runs all health checks
func (hc *HealthChecker) CheckAll(ctx context.Context) map[string]HealthStatus {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck)
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mu.RUnlock()

	results := make(map[string]HealthStatus)
	var wg sync.WaitGroup

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check HealthCheck) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
			defer cancel()

			start := time.Now()
			err := check.CheckFunc(checkCtx)
			latency := time.Since(start)

			status := HealthStatus{
				Name:    name,
				Status:  "healthy",
				Latency: latency,
			}

			if err != nil {
				status.Status = "unhealthy"
				status.Error = err.Error()
			}

			results[name] = status
		}(name, check)
	}

	wg.Wait()
	return results
}

// IsHealthy returns true if all checks are healthy
func (hc *HealthChecker) IsHealthy(ctx context.Context) bool {
	results := hc.CheckAll(ctx)
	for _, status := range results {
		if status.Status != "healthy" {
			return false
		}
	}
	return true
}