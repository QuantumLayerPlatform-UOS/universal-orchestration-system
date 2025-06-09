const io = require('socket.io-client');
const winston = require('winston');
const dotenv = require('dotenv');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');
const { MetaPromptEngine } = require('./MetaPromptEngine');
const AgentSpawner = require('./AgentSpawner');

// Load environment variables
dotenv.config();

// Configure logger
const logger = winston.createLogger({
  level: process.env.LOG_LEVEL || 'info',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.colorize(),
    winston.format.simple()
  ),
  transports: [new winston.transports.Console()]
});

// Meta-prompt agent configuration
const META_AGENT_CONFIG = {
  id: process.env.META_AGENT_ID || 'meta-prompt-orchestrator',
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
    supportsDynamicAgents: true,
    maxConcurrentAgents: 10,
    promptEngineVersion: '1.0.0'
  }
};

const AGENT_MANAGER_URL = process.env.AGENT_MANAGER_URL || 'http://localhost:8082';

class MetaPromptOrchestrator {
  constructor() {
    this.socket = null;
    this.registered = false;
    this.activeAgents = new Map();
    this.taskQueue = [];
    
    // Initialize meta-prompt engine
    this.promptEngine = new MetaPromptEngine({
      azureOpenAIApiKey: process.env.AZURE_OPENAI_API_KEY,
      azureOpenAIInstanceName: process.env.AZURE_OPENAI_INSTANCE_NAME,
      azureOpenAIDeploymentName: process.env.AZURE_OPENAI_DEPLOYMENT_NAME,
      azureOpenAIApiVersion: process.env.AZURE_OPENAI_API_VERSION
    });

    // Initialize agent spawner
    this.agentSpawner = new AgentSpawner({
      agentManagerUrl: AGENT_MANAGER_URL,
      logger: logger
    });
  }

  async connect() {
    logger.info(`Connecting to Agent Manager at ${AGENT_MANAGER_URL}`);
    
    // Register meta-prompt orchestrator
    await this.registerAgent();
    
    // Connect via Socket.IO
    this.socket = io(`${AGENT_MANAGER_URL}/agents`, {
      reconnection: true,
      reconnectionDelay: 5000,
      reconnectionAttempts: 10,
      auth: {
        token: 'meta-agent-token',
        agentId: META_AGENT_CONFIG.id
      }
    });

    this.setupEventHandlers();
  }

  async registerAgent() {
    try {
      logger.info('Registering meta-prompt orchestrator');
      const response = await axios.post(`${AGENT_MANAGER_URL}/api/v1/agents`, {
        ...META_AGENT_CONFIG,
        endpoint: `${AGENT_MANAGER_URL}/agents/${META_AGENT_CONFIG.id}`,
        region: 'global',
        tags: ['meta-prompt', 'orchestrator', 'dynamic-agents'],
        status: 'available'
      });
      
      logger.info('Meta-prompt orchestrator registered successfully');
      this.registered = true;
    } catch (error) {
      logger.error('Failed to register meta-prompt orchestrator:', error.response?.data || error.message);
      throw error;
    }
  }

  setupEventHandlers() {
    this.socket.on('connect', () => {
      logger.info('Connected to Agent Manager');
      this.updateStatus('available');
      this.startHeartbeat();
    });

    this.socket.on('disconnect', () => {
      logger.warn('Disconnected from Agent Manager');
    });

    this.socket.on('task', async (task) => {
      logger.info('Received task:', { taskId: task.id, type: task.type });
      await this.handleTask(task);
    });

    this.socket.on('meta-prompt-request', async (request) => {
      logger.info('Received meta-prompt request:', request);
      await this.handleMetaPromptRequest(request);
    });
  }

  updateStatus(status) {
    logger.info(`Updating status to: ${status}`);
    this.socket.emit('status:update', status);
  }

  startHeartbeat() {
    setInterval(() => {
      this.socket.emit('heartbeat', {
        activeAgents: this.activeAgents.size,
        queueLength: this.taskQueue.length
      });
    }, 30000);
  }

  async handleTask(task) {
    try {
      this.socket.emit('task-update', {
        taskId: task.id,
        status: 'in-progress',
        agentId: META_AGENT_CONFIG.id
      });

      let result;
      switch (task.type) {
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
        
        default:
          throw new Error(`Unknown task type: ${task.type}`);
      }

      this.socket.emit('task-complete', {
        taskId: task.id,
        agentId: META_AGENT_CONFIG.id,
        status: 'completed',
        result: result
      });
    } catch (error) {
      logger.error(`Error processing task ${task.id}:`, error);
      
      this.socket.emit('task-error', {
        taskId: task.id,
        agentId: META_AGENT_CONFIG.id,
        status: 'failed',
        error: error.message
      });
    }
  }

  async handleDesignAgent(task) {
    const { taskDescription, requirements, context } = task.payload;
    
    logger.info('Designing new agent', { taskDescription });
    
    const agentDesign = await this.promptEngine.designAgent(
      taskDescription,
      requirements,
      context
    );

    // Store the design for future spawning
    const designId = uuidv4();
    await this.storeAgentDesign(designId, agentDesign);

    return {
      designId,
      agentDesign,
      message: `Agent designed successfully: ${agentDesign.name}`
    };
  }

  async handleOptimizePrompt(task) {
    const { agentId, currentPrompt, performanceData } = task.payload;
    
    logger.info('Optimizing prompt for agent', { agentId });
    
    const optimization = await this.promptEngine.optimizePrompt(
      currentPrompt,
      performanceData
    );

    // Track the optimization
    await this.trackOptimization(agentId, optimization);

    return {
      agentId,
      optimization,
      message: 'Prompt optimized successfully'
    };
  }

  async handleSpawnAgent(task) {
    const { designId, taskContext, ttl = 3600000 } = task.payload; // Default TTL: 1 hour
    
    logger.info('Spawning dynamic agent', { designId });
    
    // Retrieve agent design
    const agentDesign = await this.getAgentDesign(designId);
    if (!agentDesign) {
      throw new Error(`Agent design not found: ${designId}`);
    }

    // Create unique agent ID
    const agentId = `dynamic-${uuidv4().substring(0, 8)}`;
    
    // Spawn the agent
    const spawnedAgent = await this.agentSpawner.spawn({
      id: agentId,
      design: agentDesign,
      context: taskContext,
      ttl
    });

    // Track the spawned agent
    this.activeAgents.set(agentId, {
      ...spawnedAgent,
      spawnedAt: new Date(),
      ttl
    });

    // Set up auto-cleanup
    setTimeout(() => {
      this.cleanupAgent(agentId);
    }, ttl);

    return {
      agentId,
      status: 'spawned',
      capabilities: agentDesign.capabilities,
      ttl,
      message: `Dynamic agent ${agentDesign.name} spawned successfully`
    };
  }

  async handleDecomposeTask(task) {
    const { taskDescription, availableAgents, constraints } = task.payload;
    
    logger.info('Decomposing complex task', { taskDescription });
    
    const workflow = await this.promptEngine.decomposeTask(
      taskDescription,
      availableAgents || this.getAvailableAgentTypes(),
      constraints
    );

    return {
      workflow,
      message: 'Task decomposed into workflow successfully'
    };
  }

  async handleMetaPromptRequest(request) {
    const { type, payload, requestId } = request;
    
    try {
      let response;
      
      switch (type) {
        case 'extract-capabilities':
          response = await this.promptEngine.extractCapabilities(
            payload.prompt,
            payload.agentType
          );
          break;
        
        case 'get-performance':
          response = this.promptEngine.getPerformanceSummary(payload.agentId);
          break;
        
        case 'list-active-agents':
          response = Array.from(this.activeAgents.values()).map(agent => ({
            id: agent.id,
            name: agent.design.name,
            type: agent.design.type,
            status: agent.status,
            spawnedAt: agent.spawnedAt,
            ttl: agent.ttl
          }));
          break;
        
        default:
          throw new Error(`Unknown meta-prompt request type: ${type}`);
      }

      this.socket.emit('meta-prompt-response', {
        requestId,
        success: true,
        response
      });
    } catch (error) {
      logger.error('Error handling meta-prompt request:', error);
      
      this.socket.emit('meta-prompt-response', {
        requestId,
        success: false,
        error: error.message
      });
    }
  }

  async storeAgentDesign(designId, design) {
    // In production, this would store in a database
    // For now, we'll keep in memory
    if (!this.agentDesigns) {
      this.agentDesigns = new Map();
    }
    this.agentDesigns.set(designId, design);
  }

  async getAgentDesign(designId) {
    return this.agentDesigns?.get(designId);
  }

  async trackOptimization(agentId, optimization) {
    // Track optimization history
    if (!this.optimizationHistory) {
      this.optimizationHistory = new Map();
    }
    
    const history = this.optimizationHistory.get(agentId) || [];
    history.push({
      timestamp: new Date(),
      optimization
    });
    
    this.optimizationHistory.set(agentId, history);
  }

  getAvailableAgentTypes() {
    // Return known agent types
    return [
      'code-gen',
      'test-gen',
      'review',
      'documentation',
      'security',
      'optimization',
      'deployment'
    ];
  }

  async cleanupAgent(agentId) {
    logger.info('Cleaning up expired agent', { agentId });
    
    const agent = this.activeAgents.get(agentId);
    if (agent) {
      // Notify agent spawner to cleanup
      await this.agentSpawner.cleanup(agentId);
      
      // Remove from active agents
      this.activeAgents.delete(agentId);
      
      // Notify agent manager
      this.socket.emit('agent-cleanup', {
        agentId,
        reason: 'TTL expired'
      });
    }
  }

  shutdown() {
    logger.info('Shutting down meta-prompt orchestrator');
    
    // Cleanup all active agents
    for (const [agentId] of this.activeAgents) {
      this.cleanupAgent(agentId);
    }
    
    if (this.socket) {
      this.socket.emit('agent-shutdown', {
        agentId: META_AGENT_CONFIG.id,
        reason: 'Orchestrator shutdown'
      });
      this.socket.disconnect();
    }
    
    process.exit(0);
  }
}

// Create and start the orchestrator
const orchestrator = new MetaPromptOrchestrator();

// Handle process signals
process.on('SIGINT', () => orchestrator.shutdown());
process.on('SIGTERM', () => orchestrator.shutdown());

// Start the orchestrator
(async () => {
  try {
    await orchestrator.connect();
    logger.info('Meta-prompt orchestrator started successfully');
  } catch (error) {
    logger.error('Failed to start meta-prompt orchestrator:', error);
    process.exit(1);
  }
})();