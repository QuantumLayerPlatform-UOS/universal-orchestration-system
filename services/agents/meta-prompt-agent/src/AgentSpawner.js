const io = require('socket.io-client');
const { v4: uuidv4 } = require('uuid');
const { DynamicAgent } = require('./MetaPromptEngine');

class AgentSpawner {
  constructor(config) {
    this.agentManagerUrl = config.agentManagerUrl;
    this.logger = config.logger;
    this.spawnedAgents = new Map();
  }

  /**
   * Spawn a new dynamic agent instance
   */
  async spawn(config) {
    const { id, design, context, ttl } = config;
    
    this.logger.info('Spawning dynamic agent', { 
      id, 
      name: design.name,
      type: design.type 
    });

    // Create agent configuration
    const agentConfig = {
      id,
      name: design.name,
      type: design.type,
      capabilities: design.capabilities,
      metadata: {
        ...design.metadata,
        isDynamic: true,
        spawnedAt: new Date().toISOString(),
        ttl,
        designVersion: design.version,
        systemPrompt: design.systemPrompt
      }
    };

    // Register the dynamic agent with agent manager
    await this.registerDynamicAgent(agentConfig);

    // Create socket connection for the agent
    const socket = this.createAgentSocket(id);

    // Create the dynamic agent instance
    const dynamicAgent = new DynamicAgentInstance({
      ...agentConfig,
      design,
      socket,
      logger: this.logger,
      context
    });

    // Store the agent instance
    this.spawnedAgents.set(id, dynamicAgent);

    // Start the agent
    await dynamicAgent.start();

    return {
      id,
      design,
      status: 'active',
      socket: socket.id
    };
  }

  /**
   * Register dynamic agent with agent manager
   */
  async registerDynamicAgent(agentConfig) {
    const axios = require('axios');
    
    try {
      const response = await axios.post(
        `${this.agentManagerUrl}/api/v1/agents`, 
        {
          ...agentConfig,
          status: 'available',
          endpoint: `dynamic-agent-${agentConfig.id}`,
          region: 'dynamic',
          tags: ['dynamic', 'meta-prompt', agentConfig.type]
        }
      );
      
      this.logger.info('Dynamic agent registered', { 
        agentId: agentConfig.id,
        response: response.data 
      });
    } catch (error) {
      this.logger.error('Failed to register dynamic agent', {
        agentId: agentConfig.id,
        error: error.response?.data || error.message
      });
      throw error;
    }
  }

  /**
   * Create socket connection for dynamic agent
   */
  createAgentSocket(agentId) {
    const socket = io(`${this.agentManagerUrl}/agents`, {
      reconnection: true,
      reconnectionDelay: 5000,
      auth: {
        token: 'dynamic-agent-token',
        agentId
      }
    });

    return socket;
  }

  /**
   * Cleanup a dynamic agent
   */
  async cleanup(agentId) {
    const agent = this.spawnedAgents.get(agentId);
    
    if (agent) {
      this.logger.info('Cleaning up dynamic agent', { agentId });
      
      // Stop the agent
      await agent.stop();
      
      // Remove from registry
      this.spawnedAgents.delete(agentId);
      
      // Unregister from agent manager
      await this.unregisterAgent(agentId);
    }
  }

  /**
   * Unregister agent from agent manager
   */
  async unregisterAgent(agentId) {
    const axios = require('axios');
    
    try {
      await axios.delete(`${this.agentManagerUrl}/api/v1/agents/${agentId}`);
      this.logger.info('Dynamic agent unregistered', { agentId });
    } catch (error) {
      this.logger.error('Failed to unregister dynamic agent', {
        agentId,
        error: error.response?.data || error.message
      });
    }
  }

  /**
   * Get status of all spawned agents
   */
  getStatus() {
    const status = [];
    
    for (const [id, agent] of this.spawnedAgents) {
      status.push({
        id,
        name: agent.name,
        type: agent.type,
        status: agent.status,
        tasksProcessed: agent.tasksProcessed,
        uptime: Date.now() - agent.startTime
      });
    }
    
    return status;
  }
}

/**
 * Dynamic Agent Instance that handles task execution
 */
class DynamicAgentInstance {
  constructor(config) {
    this.id = config.id;
    this.name = config.name;
    this.type = config.type;
    this.capabilities = config.capabilities;
    this.design = config.design;
    this.socket = config.socket;
    this.logger = config.logger;
    this.context = config.context;
    
    this.status = 'initializing';
    this.tasksProcessed = 0;
    this.startTime = Date.now();
    this.currentTask = null;
    
    // Create the actual dynamic agent
    this.agent = new DynamicAgent({
      id: this.id,
      design: this.design,
      taskQueue: [],
      llm: this.createLLM(),
      logger: this.logger
    });
  }

  /**
   * Start the dynamic agent
   */
  async start() {
    this.setupSocketHandlers();
    this.status = 'available';
    this.updateStatus('available');
    this.startHeartbeat();
    
    this.logger.info('Dynamic agent started', { 
      id: this.id, 
      name: this.name 
    });
  }

  /**
   * Stop the dynamic agent
   */
  async stop() {
    this.status = 'stopping';
    
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
    }
    
    if (this.socket) {
      this.socket.emit('agent-shutdown', {
        agentId: this.id,
        reason: 'Dynamic agent cleanup'
      });
      this.socket.disconnect();
    }
    
    this.status = 'stopped';
    this.logger.info('Dynamic agent stopped', { id: this.id });
  }

  /**
   * Set up socket event handlers
   */
  setupSocketHandlers() {
    this.socket.on('connect', () => {
      this.logger.info('Dynamic agent connected', { id: this.id });
      this.updateStatus('available');
    });

    this.socket.on('disconnect', () => {
      this.logger.warn('Dynamic agent disconnected', { id: this.id });
    });

    this.socket.on('task', async (task) => {
      await this.handleTask(task);
    });

    this.socket.on('heartbeat:ack', () => {
      this.logger.debug('Heartbeat acknowledged', { id: this.id });
    });
  }

  /**
   * Handle incoming task
   */
  async handleTask(task) {
    this.currentTask = task;
    this.status = 'busy';
    this.updateStatus('busy');
    
    try {
      this.logger.info('Dynamic agent processing task', { 
        agentId: this.id,
        taskId: task.id,
        taskType: task.type 
      });
      
      // Update task status
      this.socket.emit('task-update', {
        taskId: task.id,
        status: 'in-progress',
        agentId: this.id
      });

      // Execute task using the dynamic agent
      const result = await this.agent.executeTask({
        id: task.id,
        input: task.payload,
        context: { ...this.context, ...task.context }
      });

      // Send result back
      if (result.success) {
        this.socket.emit('task-complete', {
          taskId: task.id,
          agentId: this.id,
          status: 'completed',
          result: result.result,
          metrics: result.metrics
        });
      } else {
        throw new Error(result.error);
      }

      this.tasksProcessed++;
      this.logger.info('Dynamic agent task completed', { 
        agentId: this.id,
        taskId: task.id 
      });
    } catch (error) {
      this.logger.error('Dynamic agent task failed', { 
        agentId: this.id,
        taskId: task.id,
        error: error.message 
      });
      
      this.socket.emit('task-error', {
        taskId: task.id,
        agentId: this.id,
        status: 'failed',
        error: error.message
      });
    } finally {
      this.currentTask = null;
      this.status = 'available';
      this.updateStatus('available');
    }
  }

  /**
   * Update agent status
   */
  updateStatus(status) {
    this.socket.emit('status:update', status);
  }

  /**
   * Start heartbeat
   */
  startHeartbeat() {
    this.heartbeatInterval = setInterval(() => {
      this.socket.emit('heartbeat', {
        tasksProcessed: this.tasksProcessed,
        uptime: Date.now() - this.startTime,
        currentTask: this.currentTask?.id || null
      });
    }, 30000);
  }

  /**
   * Create LLM instance for the agent
   */
  createLLM() {
    const { ChatOpenAI } = require('@langchain/openai');
    
    return new ChatOpenAI({
      azureOpenAIApiKey: process.env.AZURE_OPENAI_API_KEY,
      azureOpenAIApiInstanceName: process.env.AZURE_OPENAI_INSTANCE_NAME,
      azureOpenAIApiDeploymentName: process.env.AZURE_OPENAI_DEPLOYMENT_NAME,
      azureOpenAIApiVersion: process.env.AZURE_OPENAI_API_VERSION || '2023-05-15',
      ...this.design.behaviorModifiers
    });
  }
}

module.exports = AgentSpawner;