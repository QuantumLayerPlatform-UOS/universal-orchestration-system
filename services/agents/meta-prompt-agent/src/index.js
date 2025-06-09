const ResilientMetaPromptAgent = require('./ResilientMetaPromptAgent');
const dotenv = require('dotenv');

// Load environment variables
dotenv.config();

// Create and start the resilient meta-prompt agent
const agent = new ResilientMetaPromptAgent({
  agentId: process.env.META_AGENT_ID || 'meta-prompt-orchestrator',
  agentManagerUrl: process.env.AGENT_MANAGER_URL || 'http://localhost:8082',
  
  // Resilience configurations
  maxRetries: parseInt(process.env.MAX_RETRIES || '3'),
  retryDelay: parseInt(process.env.RETRY_DELAY || '1000'),
  retryBackoffMultiplier: parseFloat(process.env.RETRY_BACKOFF_MULTIPLIER || '2'),
  circuitBreakerThreshold: parseInt(process.env.CIRCUIT_BREAKER_THRESHOLD || '5'),
  circuitBreakerTimeout: parseInt(process.env.CIRCUIT_BREAKER_TIMEOUT || '60000'),
  healthCheckInterval: parseInt(process.env.HEALTH_CHECK_INTERVAL || '30000'),
  requestTimeout: parseInt(process.env.REQUEST_TIMEOUT || '30000')
});

// Export the agent instance for testing
module.exports = agent;