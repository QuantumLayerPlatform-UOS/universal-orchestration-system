const EventEmitter = require('events');
const winston = require('winston');
const axios = require('axios');

/**
 * Base class for creating resilient agents with built-in error handling,
 * retry logic, circuit breakers, and health monitoring
 */
class ResilientAgentBase extends EventEmitter {
  constructor(config = {}) {
    super();
    
    this.config = {
      maxRetries: config.maxRetries || 3,
      retryDelay: config.retryDelay || 1000,
      retryBackoffMultiplier: config.retryBackoffMultiplier || 2,
      circuitBreakerThreshold: config.circuitBreakerThreshold || 5,
      circuitBreakerTimeout: config.circuitBreakerTimeout || 60000,
      healthCheckInterval: config.healthCheckInterval || 30000,
      requestTimeout: config.requestTimeout || 30000,
      ...config
    };
    
    this.logger = this.setupLogger();
    this.circuitBreakers = new Map();
    this.healthStatus = {
      status: 'initializing',
      lastCheck: new Date(),
      errors: [],
      metrics: {}
    };
    
    this.setupHealthMonitoring();
    this.setupGracefulShutdown();
  }
  
  setupLogger() {
    return winston.createLogger({
      level: process.env.LOG_LEVEL || 'info',
      format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.errors({ stack: true }),
        winston.format.json()
      ),
      defaultMeta: { 
        service: this.config.serviceName || 'resilient-agent',
        agentId: this.config.agentId
      },
      transports: [
        new winston.transports.Console({
          format: winston.format.combine(
            winston.format.colorize(),
            winston.format.simple()
          )
        })
      ]
    });
  }
  
  /**
   * Execute a function with retry logic
   */
  async withRetry(fn, context = {}) {
    let lastError;
    
    for (let attempt = 0; attempt <= this.config.maxRetries; attempt++) {
      try {
        return await fn();
      } catch (error) {
        lastError = error;
        
        if (attempt < this.config.maxRetries) {
          const delay = this.config.retryDelay * Math.pow(this.config.retryBackoffMultiplier, attempt);
          
          this.logger.warn(`Retry attempt ${attempt + 1}/${this.config.maxRetries} after ${delay}ms`, {
            error: error.message,
            context
          });
          
          await this.sleep(delay);
        }
      }
    }
    
    this.logger.error('All retry attempts failed', {
      error: lastError.message,
      context
    });
    
    throw lastError;
  }
  
  /**
   * Execute a function with circuit breaker protection
   */
  async withCircuitBreaker(key, fn) {
    let breaker = this.circuitBreakers.get(key);
    
    if (!breaker) {
      breaker = {
        failures: 0,
        lastFailure: null,
        state: 'closed' // closed, open, half-open
      };
      this.circuitBreakers.set(key, breaker);
    }
    
    // Check if circuit is open
    if (breaker.state === 'open') {
      const timeSinceFailure = Date.now() - breaker.lastFailure;
      
      if (timeSinceFailure < this.config.circuitBreakerTimeout) {
        throw new Error(`Circuit breaker is open for ${key}`);
      } else {
        // Try half-open
        breaker.state = 'half-open';
      }
    }
    
    try {
      const result = await fn();
      
      // Reset on success
      if (breaker.state === 'half-open') {
        breaker.state = 'closed';
        breaker.failures = 0;
        this.logger.info(`Circuit breaker closed for ${key}`);
      }
      
      return result;
    } catch (error) {
      breaker.failures++;
      breaker.lastFailure = Date.now();
      
      if (breaker.failures >= this.config.circuitBreakerThreshold) {
        breaker.state = 'open';
        this.logger.error(`Circuit breaker opened for ${key}`, {
          failures: breaker.failures
        });
      }
      
      throw error;
    }
  }
  
  /**
   * Make HTTP request with timeout and error handling
   */
  async request(options) {
    const controller = new AbortController();
    const timeout = setTimeout(() => {
      controller.abort();
    }, this.config.requestTimeout);
    
    try {
      const response = await axios({
        ...options,
        signal: controller.signal,
        timeout: this.config.requestTimeout,
        validateStatus: (status) => status < 500
      });
      
      return response;
    } catch (error) {
      if (error.code === 'ECONNABORTED' || error.message.includes('timeout')) {
        throw new Error(`Request timeout after ${this.config.requestTimeout}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }
  
  /**
   * Setup health monitoring
   */
  setupHealthMonitoring() {
    setInterval(async () => {
      try {
        await this.performHealthCheck();
      } catch (error) {
        this.logger.error('Health check failed', { error: error.message });
      }
    }, this.config.healthCheckInterval);
  }
  
  /**
   * Perform health check - override in subclasses
   */
  async performHealthCheck() {
    const checks = {
      memory: this.checkMemoryUsage(),
      eventLoop: await this.checkEventLoopDelay(),
      custom: await this.performCustomHealthChecks()
    };
    
    const allHealthy = Object.values(checks).every(check => check.healthy);
    
    this.healthStatus = {
      status: allHealthy ? 'healthy' : 'unhealthy',
      lastCheck: new Date(),
      checks,
      errors: this.healthStatus.errors.slice(-10) // Keep last 10 errors
    };
    
    return this.healthStatus;
  }
  
  /**
   * Check memory usage
   */
  checkMemoryUsage() {
    const usage = process.memoryUsage();
    const heapUsedPercent = (usage.heapUsed / usage.heapTotal) * 100;
    
    return {
      healthy: heapUsedPercent < 90,
      heapUsedPercent,
      rss: usage.rss,
      heapTotal: usage.heapTotal,
      heapUsed: usage.heapUsed
    };
  }
  
  /**
   * Check event loop delay
   */
  async checkEventLoopDelay() {
    const start = Date.now();
    
    await new Promise(resolve => setImmediate(resolve));
    
    const delay = Date.now() - start;
    
    return {
      healthy: delay < 100,
      delay
    };
  }
  
  /**
   * Override this in subclasses for custom health checks
   */
  async performCustomHealthChecks() {
    return { healthy: true };
  }
  
  /**
   * Log error and add to health status
   */
  logError(error, context = {}) {
    this.logger.error(error.message, {
      ...context,
      stack: error.stack
    });
    
    this.healthStatus.errors.push({
      timestamp: new Date(),
      message: error.message,
      context
    });
  }
  
  /**
   * Setup graceful shutdown
   */
  setupGracefulShutdown() {
    const shutdown = async (signal) => {
      this.logger.info(`Received ${signal}, starting graceful shutdown`);
      
      try {
        await this.cleanup();
        process.exit(0);
      } catch (error) {
        this.logger.error('Error during shutdown', { error: error.message });
        process.exit(1);
      }
    };
    
    process.on('SIGTERM', () => shutdown('SIGTERM'));
    process.on('SIGINT', () => shutdown('SIGINT'));
    
    process.on('uncaughtException', (error) => {
      this.logger.error('Uncaught exception', { error: error.message, stack: error.stack });
      shutdown('uncaughtException');
    });
    
    process.on('unhandledRejection', (reason, promise) => {
      this.logger.error('Unhandled rejection', { reason, promise });
    });
  }
  
  /**
   * Override this in subclasses for cleanup logic
   */
  async cleanup() {
    this.logger.info('Cleaning up resources');
  }
  
  /**
   * Utility sleep function
   */
  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
  
  /**
   * Get current health status
   */
  getHealthStatus() {
    return this.healthStatus;
  }
}

module.exports = ResilientAgentBase;