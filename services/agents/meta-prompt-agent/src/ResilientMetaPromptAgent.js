const ResilientAgentBase = require('./ResilientAgentBase');
const io = require('socket.io-client');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');
const { MetaPromptEngine } = require('./MetaPromptEngine');
const AgentSpawner = require('./AgentSpawner');
const Joi = require('joi');

// Validation schemas
const schemas = {
  agentDesignRequest: Joi.object({
    taskDescription: Joi.string().required().min(10).max(1000),
    requirements: Joi.object().optional(),
    context: Joi.object().optional()
  }),
  
  spawnAgentRequest: Joi.object({
    designId: Joi.string().uuid().required(),
    taskContext: Joi.object().optional(),
    ttl: Joi.number().min(60000).max(86400000).optional() // 1 min to 24 hours
  }),
  
  task: Joi.object({
    id: Joi.string().required(),
    type: Joi.string().required(),
    payload: Joi.object().required()
  })
};

class ResilientMetaPromptAgent extends ResilientAgentBase {
  constructor(config = {}) {
    super({
      serviceName: 'meta-prompt-agent',
      agentId: config.agentId || 'meta-prompt-orchestrator',
      ...config
    });
    
    this.agentConfig = {
      id: this.config.agentId,
      name: 'Meta-Prompt Orchestrator',
      type: 'meta-prompt',
      capabilities: [
        {
          name: 'design-agent',
          description: 'Design new agents based on requirements',
          version: '1.0.0'
        },
        {
          name: 'optimize-prompt',
          description: 'Optimize existing agent prompts',
          version: '1.0.0'
        },
        {
          name: 'spawn-agent',
          description: 'Spawn dynamic agents on demand',
          version: '1.0.0'
        },
        {
          name: 'decompose-task',
          description: 'Decompose complex tasks into workflows',
          version: '1.0.0'
        },
        {
          name: 'monitor-performance',
          description: 'Monitor and improve agent performance',
          version: '1.0.0'
        }
      ],
      metadata: {
        version: '1.0.0',
        platform: 'nodejs',
        supportsDynamicAgents: true,
        maxConcurrentAgents: 10,
        promptEngineVersion: '1.0.0'
      }
    };
    
    this.agentManagerUrl = config.agentManagerUrl || process.env.AGENT_MANAGER_URL || 'http://localhost:8082';
    this.socket = null;
    this.registered = false;
    this.activeAgents = new Map();
    this.taskQueue = [];
    this.connectionRetries = 0;
    this.maxConnectionRetries = 10;
    
    // Initialize components
    this.promptEngine = null;
    this.agentSpawner = null;
    
    // Agent designs storage (in production, use persistent storage)
    this.agentDesigns = new Map();
    
    // Start the agent
    this.start().catch(error => {
      this.logger.error('Failed to start agent', { error: error.message });
      process.exit(1);
    });
  }
  
  async start() {
    try {
      this.logger.info('Starting Meta-Prompt Orchestrator');
      
      // Initialize prompt engine
      await this.initializePromptEngine();
      
      // Initialize agent spawner
      this.agentSpawner = new AgentSpawner({
        agentManagerUrl: this.agentManagerUrl,
        logger: this.logger
      });
      
      // Connect to agent manager
      await this.connectToAgentManager();
      
      this.logger.info('Meta-Prompt Orchestrator started successfully');
    } catch (error) {
      this.logError(error, { phase: 'startup' });
      throw error;
    }
  }
  
  async initializePromptEngine() {
    try {
      this.promptEngine = new MetaPromptEngine({
        llmProvider: process.env.LLM_PROVIDER,
        llmModel: process.env.LLM_MODEL,
        azureOpenAIApiKey: process.env.AZURE_OPENAI_API_KEY,
        azureOpenAIInstanceName: process.env.AZURE_OPENAI_INSTANCE_NAME,
        azureOpenAIDeploymentName: process.env.AZURE_OPENAI_DEPLOYMENT_NAME,
        azureOpenAIApiVersion: process.env.AZURE_OPENAI_API_VERSION,
        temperature: parseFloat(process.env.LLM_TEMPERATURE || '0.7'),
        maxTokens: parseInt(process.env.LLM_MAX_TOKENS || '2000')
      });
      
      this.logger.info('Prompt engine initialized');
    } catch (error) {
      this.logger.error('Failed to initialize prompt engine', { error: error.message });
      throw error;
    }
  }
  
  async connectToAgentManager() {
    return this.withRetry(async () => {
      await this.registerAgent();
      await this.setupSocketConnection();
    }, { operation: 'connectToAgentManager' });
  }
  
  async registerAgent() {
    try {
      this.logger.info('Registering with Agent Manager');
      
      // Check if already registered
      const agents = await this.request({
        method: 'GET',
        url: `${this.agentManagerUrl}/api/v1/agents`
      });
      
      const existingAgent = agents.data.agents?.find(a => a.id === this.agentConfig.id);
      
      if (existingAgent) {
        this.logger.info('Agent already registered');
        this.registered = true;
        return;
      }
      
      // Register new agent
      await this.request({
        method: 'POST',
        url: `${this.agentManagerUrl}/api/v1/agents`,
        data: {
          id: this.agentConfig.id,
          name: this.agentConfig.name,
          type: this.agentConfig.type,
          capabilities: this.agentConfig.capabilities,
          endpoint: `${this.agentManagerUrl}/agents/${this.agentConfig.id}`,
          metadata: {
            ...this.agentConfig.metadata,
            region: 'global',
            tags: ['meta-prompt', 'orchestrator', 'dynamic-agents']
          },
          status: 'available'
        }
      });
      
      this.logger.info('Agent registered successfully');
      this.registered = true;
      
    } catch (error) {
      if (error.response?.status === 409 || error.response?.data?.error?.code === 11000 || error.response?.data?.code === 11000) {
        this.logger.info('Agent already exists in database, continuing');
        this.registered = true;
      } else {
        throw error;
      }
    }
  }
  
  async setupSocketConnection() {
    return new Promise((resolve, reject) => {
      this.logger.info('Setting up Socket.IO connection');
      
      this.socket = io(`${this.agentManagerUrl}/agents`, {
        reconnection: true,
        reconnectionDelay: 5000,
        reconnectionAttempts: this.maxConnectionRetries,
        auth: {
          token: process.env.AGENT_AUTH_TOKEN || 'meta-agent-token',
          agentId: this.agentConfig.id
        }
      });
      
      this.socket.on('connect', () => {
        this.logger.info('Connected to Agent Manager via Socket.IO');
        this.connectionRetries = 0;
        
        // Store agent info in socket data for auto-registration
        this.socket.data = this.socket.data || {};
        this.socket.data.agentInfo = {
          id: this.agentConfig.id,
          name: this.agentConfig.name,
          type: this.agentConfig.type,
          capabilities: this.agentConfig.capabilities,
          endpoint: `${this.agentManagerUrl}/agents/${this.agentConfig.id}`,
          metadata: {
            ...this.agentConfig.metadata,
            region: 'global',
            tags: ['meta-prompt', 'orchestrator', 'dynamic-agents']
          }
        };
        
        // Emit agent info
        this.socket.emit('agent:info', this.socket.data.agentInfo);
        
        this.updateStatus('available');
        this.startHeartbeat();
        resolve();
      });
      
      this.socket.on('disconnect', (reason) => {
        this.logger.warn('Disconnected from Agent Manager', { reason });
        
        // Don't reject on disconnect if we already connected once
        if (this.registered && reason !== 'io client disconnect') {
          // Server-side disconnect, will auto-reconnect
          this.logger.info('Will attempt to reconnect automatically');
        }
      });
      
      this.socket.on('connect_error', (error) => {
        this.connectionRetries++;
        this.logger.error('Socket connection error', { 
          error: error.message,
          type: error.type,
          data: error.data,
          retries: this.connectionRetries
        });
        
        if (this.connectionRetries >= this.maxConnectionRetries) {
          reject(new Error('Failed to connect to Agent Manager'));
        }
      });
      
      this.setupSocketEventHandlers();
      
      // Handle agent:info:request from server
      this.socket.on('agent:info:request', () => {
        this.logger.info('Server requested agent info');
        
        const agentInfo = {
          id: this.agentConfig.id,
          name: this.agentConfig.name,
          type: this.agentConfig.type,
          capabilities: this.agentConfig.capabilities,
          endpoint: `${this.agentManagerUrl}/agents/${this.agentConfig.id}`,
          metadata: {
            ...this.agentConfig.metadata,
            region: 'global',
            tags: ['meta-prompt', 'orchestrator', 'dynamic-agents']
          }
        };
        
        this.socket.emit('agent:info', agentInfo);
      });
      
      // Add reconnection event handlers
      this.socket.on('reconnect', (attemptNumber) => {
        this.logger.info('Reconnected to Agent Manager', { attemptNumber });
        this.updateStatus('available');
      });
      
      this.socket.on('reconnect_attempt', (attemptNumber) => {
        this.logger.debug('Attempting to reconnect', { attemptNumber });
      });
      
      this.socket.on('reconnect_error', (error) => {
        this.logger.warn('Reconnection error', { error: error.message });
      });
      
      this.socket.on('reconnect_failed', () => {
        this.logger.error('Failed to reconnect after maximum attempts');
      });
      
      // Timeout for initial connection
      setTimeout(() => {
        if (!this.socket.connected) {
          reject(new Error('Socket connection timeout'));
        }
      }, 30000);
    });
  }
  
  setupSocketEventHandlers() {
    // Task handling
    this.socket.on('task', async (task) => {
      try {
        const validation = schemas.task.validate(task);
        if (validation.error) {
          throw new Error(`Invalid task format: ${validation.error.message}`);
        }
        
        this.logger.info('Received task', { taskId: task.id, type: task.type });
        await this.handleTask(task);
      } catch (error) {
        this.logError(error, { taskId: task?.id });
      }
    });
    
    // Meta-prompt specific requests
    this.socket.on('meta-prompt-request', async (request) => {
      try {
        this.logger.info('Received meta-prompt request', { requestId: request.id });
        await this.handleMetaPromptRequest(request);
      } catch (error) {
        this.logError(error, { requestId: request?.id });
      }
    });
    
    // Health check requests
    this.socket.on('health-check', async () => {
      const health = await this.performHealthCheck();
      this.socket.emit('health-status', health);
    });
  }
  
  async handleTask(task) {
    const startTime = Date.now();
    
    try {
      // Emit task update
      this.emitTaskUpdate(task.id, 'in-progress');
      
      // Route task to appropriate handler
      let result;
      switch (task.payload.type || task.type) {
        case 'design-agent':
          result = await this.handleDesignAgent(task);
          break;
          
        case 'optimize-prompt':
          result = await this.handleOptimizePrompt(task);
          break;
          
        case 'spawn-agent':
          result = await this.handleSpawnAgent(task);
          break;
          
        case 'decompose-task':
          result = await this.handleDecomposeTask(task);
          break;
          
        case 'monitor-performance':
          result = await this.handleMonitorPerformance(task);
          break;
          
        default:
          throw new Error(`Unsupported task type: ${task.type}`);
      }
      
      // Emit success
      this.emitTaskComplete(task.id, result, Date.now() - startTime);
      
    } catch (error) {
      this.logError(error, { taskId: task.id });
      this.emitTaskError(task.id, error);
    }
  }
  
  async handleDesignAgent(task) {
    const validation = schemas.agentDesignRequest.validate(task.payload);
    if (validation.error) {
      throw new Error(`Invalid design request: ${validation.error.message}`);
    }
    
    const { taskDescription, requirements, context } = task.payload;
    
    this.logger.info('Designing new agent', { taskDescription });
    
    // Use circuit breaker for LLM calls
    const agentDesign = await this.withCircuitBreaker('llm-design', async () => {
      return await this.promptEngine.designAgent(
        taskDescription,
        requirements,
        context
      );
    });
    
    // Store the design
    const designId = uuidv4();
    this.agentDesigns.set(designId, {
      design: agentDesign,
      createdAt: new Date(),
      taskId: task.id
    });
    
    this.logger.info('Agent design completed', { designId });
    
    return {
      designId,
      agentDesign,
      estimatedCapabilities: agentDesign.capabilities?.length || 0
    };
  }
  
  async handleSpawnAgent(task) {
    const validation = schemas.spawnAgentRequest.validate(task.payload);
    if (validation.error) {
      throw new Error(`Invalid spawn request: ${validation.error.message}`);
    }
    
    const { designId, taskContext, ttl } = task.payload;
    
    // Retrieve design
    const designData = this.agentDesigns.get(designId);
    if (!designData) {
      throw new Error(`Design not found: ${designId}`);
    }
    
    this.logger.info('Spawning agent from design', { designId });
    
    // Spawn the agent
    const spawnResult = await this.agentSpawner.spawn({
      id: `dynamic-${uuidv4().substring(0, 8)}`,
      design: designData.design,
      context: taskContext,
      ttl
    });
    
    // Track active agent
    this.activeAgents.set(spawnResult.agentId, {
      designId,
      spawnedAt: new Date(),
      ttl,
      ...spawnResult
    });
    
    // Schedule cleanup
    if (ttl) {
      setTimeout(() => {
        this.cleanupAgent(spawnResult.agentId);
      }, ttl);
    }
    
    return spawnResult;
  }
  
  async handleOptimizePrompt(task) {
    const { currentPrompt, performanceData, targetMetrics } = task.payload;
    
    this.logger.info('Optimizing prompt');
    
    const optimizedPrompt = await this.withCircuitBreaker('llm-optimize', async () => {
      return await this.promptEngine.optimizePrompt(
        currentPrompt,
        performanceData,
        targetMetrics
      );
    });
    
    return {
      originalPrompt: currentPrompt,
      optimizedPrompt,
      expectedImprovement: optimizedPrompt.expectedImprovement
    };
  }
  
  async handleDecomposeTask(task) {
    const { taskDescription, constraints } = task.payload;
    
    this.logger.info('Decomposing task into workflow');
    
    const workflow = await this.withCircuitBreaker('llm-decompose', async () => {
      return await this.promptEngine.decomposeTask(taskDescription, constraints);
    });
    
    return {
      workflow,
      estimatedSteps: workflow.steps?.length || 0,
      estimatedDuration: workflow.estimatedDuration
    };
  }
  
  async handleMonitorPerformance(task) {
    const { agentId, metrics, threshold } = task.payload;
    
    this.logger.info('Monitoring agent performance', { agentId });
    
    const analysis = await this.promptEngine.analyzePerformance(metrics);
    
    const recommendations = [];
    
    if (analysis.performance < threshold) {
      recommendations.push({
        type: 'optimization',
        priority: 'high',
        suggestion: analysis.optimizationSuggestions
      });
    }
    
    return {
      agentId,
      performanceScore: analysis.performance,
      recommendations,
      metrics: analysis.detailedMetrics
    };
  }
  
  async handleMetaPromptRequest(request) {
    try {
      const { type, payload, respondTo } = request;
      
      let response;
      switch (type) {
        case 'list-designs':
          response = await this.listAgentDesigns();
          break;
          
        case 'get-design':
          response = await this.getAgentDesign(payload.designId);
          break;
          
        case 'list-active-agents':
          response = await this.listActiveAgents();
          break;
          
        default:
          throw new Error(`Unknown request type: ${type}`);
      }
      
      this.socket.emit(respondTo || 'meta-prompt-response', {
        requestId: request.id,
        response
      });
      
    } catch (error) {
      this.socket.emit('meta-prompt-error', {
        requestId: request.id,
        error: error.message
      });
    }
  }
  
  async listAgentDesigns() {
    const designs = [];
    
    for (const [id, data] of this.agentDesigns) {
      designs.push({
        id,
        name: data.design.name,
        type: data.design.type,
        createdAt: data.createdAt,
        capabilities: data.design.capabilities?.length || 0
      });
    }
    
    return { designs, count: designs.length };
  }
  
  async getAgentDesign(designId) {
    const design = this.agentDesigns.get(designId);
    
    if (!design) {
      throw new Error(`Design not found: ${designId}`);
    }
    
    return design;
  }
  
  async listActiveAgents() {
    const agents = [];
    
    for (const [id, data] of this.activeAgents) {
      agents.push({
        id,
        designId: data.designId,
        spawnedAt: data.spawnedAt,
        ttl: data.ttl,
        remainingTtl: data.ttl ? Math.max(0, data.ttl - (Date.now() - data.spawnedAt.getTime())) : null
      });
    }
    
    return { agents, count: agents.length };
  }
  
  async cleanupAgent(agentId) {
    try {
      this.logger.info('Cleaning up agent', { agentId });
      
      // Remove from tracking
      this.activeAgents.delete(agentId);
      
      // Notify agent manager
      await this.request({
        method: 'DELETE',
        url: `${this.agentManagerUrl}/api/v1/agents/${agentId}`
      });
      
    } catch (error) {
      this.logger.error('Failed to cleanup agent', { 
        agentId, 
        error: error.message 
      });
    }
  }
  
  updateStatus(status) {
    if (this.socket && this.socket.connected) {
      this.socket.emit('status:update', {
        agentId: this.agentConfig.id,
        status,
        timestamp: new Date()
      });
    }
  }
  
  startHeartbeat() {
    const heartbeatInterval = setInterval(() => {
      if (!this.socket || !this.socket.connected) {
        clearInterval(heartbeatInterval);
        return;
      }
      
      this.socket.emit('heartbeat', {
        agentId: this.agentConfig.id,
        activeAgents: this.activeAgents.size,
        queueLength: this.taskQueue.length,
        health: this.healthStatus.status,
        uptime: process.uptime()
      });
    }, 30000);
  }
  
  emitTaskUpdate(taskId, status, progress = null) {
    if (this.socket && this.socket.connected) {
      this.socket.emit('task:update', {
        taskId,
        agentId: this.agentConfig.id,
        status,
        progress,
        timestamp: new Date()
      });
    }
  }
  
  emitTaskComplete(taskId, result, duration) {
    if (this.socket && this.socket.connected) {
      this.socket.emit('task:complete', {
        taskId,
        agentId: this.agentConfig.id,
        status: 'completed',
        result,
        duration,
        timestamp: new Date()
      });
    }
  }
  
  emitTaskError(taskId, error) {
    if (this.socket && this.socket.connected) {
      this.socket.emit('task:error', {
        taskId,
        agentId: this.agentConfig.id,
        status: 'failed',
        error: {
          message: error.message,
          code: error.code || 'UNKNOWN_ERROR'
        },
        timestamp: new Date()
      });
    }
  }
  
  // Override health check for custom checks
  async performCustomHealthChecks() {
    const checks = {
      promptEngine: !!this.promptEngine,
      socketConnected: this.socket?.connected || false,
      registered: this.registered,
      activeAgents: this.activeAgents.size,
      taskQueueLength: this.taskQueue.length
    };
    
    const healthy = checks.promptEngine && checks.socketConnected && checks.registered;
    
    return {
      healthy,
      ...checks
    };
  }
  
  // Override cleanup for graceful shutdown
  async cleanup() {
    this.logger.info('Cleaning up Meta-Prompt Agent');
    
    // Disconnect socket
    if (this.socket) {
      this.socket.disconnect();
    }
    
    // Clean up active agents
    for (const [agentId] of this.activeAgents) {
      await this.cleanupAgent(agentId);
    }
    
    // Update status
    try {
      await this.request({
        method: 'PATCH',
        url: `${this.agentManagerUrl}/api/v1/agents/${this.agentConfig.id}/status`,
        data: { status: 'offline' }
      });
    } catch (error) {
      this.logger.error('Failed to update status on shutdown', { error: error.message });
    }
  }
}

module.exports = ResilientMetaPromptAgent;