const io = require('socket.io-client');
const winston = require('winston');
const dotenv = require('dotenv');
const axios = require('axios');
const { CodeGenerator } = require('./codeGenerator');

// Load environment variables
dotenv.config();

// Configure logger
const logger = winston.createLogger({
  level: 'info',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.colorize(),
    winston.format.simple()
  ),
  transports: [
    new winston.transports.Console()
  ]
});

// Agent configuration
const AGENT_CONFIG = {
  id: process.env.AGENT_ID || 'code-gen-agent-001',
  name: 'Code Generation Agent',
  type: 'code-gen',  // Must match the enum
  capabilities: [
    {
      name: 'generate-code',
      description: 'Generate code from specifications',
      version: '1.0.0'
    },
    {
      name: 'create-boilerplate',
      description: 'Create project boilerplate',
      version: '1.0.0'
    },
    {
      name: 'template-generation',
      description: 'Generate code from templates',
      version: '1.0.0'
    },
    {
      name: 'javascript-support',
      description: 'Generate JavaScript code',
      version: '1.0.0'
    },
    {
      name: 'multi-language-support',
      description: 'Support for multiple programming languages',
      version: '1.0.0'
    }
  ],
  status: 'idle'
};

// Agent Manager connection configuration
const AGENT_MANAGER_URL = process.env.AGENT_MANAGER_URL || 'http://localhost:8001';

class CodeGenAgent {
  constructor() {
    this.socket = null;
    this.codeGenerator = new CodeGenerator();
    this.currentTask = null;
    this.registered = false;
  }

  async connect() {
    logger.info(`Connecting to Agent Manager at ${AGENT_MANAGER_URL}`);
    
    // First, register the agent via HTTP
    await this.registerAgent();
    
    // Then connect via Socket.IO
    this.socket = io(`${AGENT_MANAGER_URL}/agents`, {
      reconnection: true,
      reconnectionDelay: 5000,
      reconnectionAttempts: 10,
      auth: {
        token: 'agent-token',
        agentId: AGENT_CONFIG.id
      }
    });

    this.setupEventHandlers();
  }

  async registerAgent() {
    try {
      logger.info('Registering agent via HTTP');
      const response = await axios.post(`${AGENT_MANAGER_URL}/api/v1/agents`, {
        ...AGENT_CONFIG,
        endpoint: `${AGENT_MANAGER_URL}/agents/${AGENT_CONFIG.id}`,
        region: 'local',
        tags: ['code-generation', 'templates', 'boilerplate'],
        metadata: {
          version: '1.0.0',
          language: 'javascript',
          platform: 'node'
        }
      });
      
      logger.info('Agent registered successfully:', response.data);
      this.registered = true;
    } catch (error) {
      logger.error('Failed to register agent:', error.response?.data || error.message);
      throw error;
    }
  }

  setupEventHandlers() {
    this.socket.on('connect', () => {
      logger.info('Connected to Agent Manager');
      // No need to register, agent-manager will handle it
      this.updateStatus('available');
      this.startHeartbeat();
    });

    this.socket.on('disconnect', () => {
      logger.warn('Disconnected from Agent Manager');
    });

    this.socket.on('welcome', (data) => {
      logger.info('Received welcome message:', data);
    });

    this.socket.on('task', async (task) => {
      logger.info('Received task assignment:', task);
      await this.handleTask(task);
    });

    this.socket.on('error', (error) => {
      logger.error('Socket error:', error);
    });

    this.socket.on('heartbeat:ack', () => {
      logger.debug('Heartbeat acknowledged');
    });
  }

  updateStatus(status) {
    logger.info(`Updating status to: ${status}`);
    this.socket.emit('status:update', status);
    AGENT_CONFIG.status = status;
  }

  startHeartbeat() {
    setInterval(() => {
      this.socket.emit('heartbeat');
    }, 30000); // Every 30 seconds
  }

  async handleTask(task) {
    this.currentTask = task;
    AGENT_CONFIG.status = 'busy';
    
    try {
      logger.info(`Processing task: ${task.id} - ${task.type}`);
      
      // Update task status to in-progress
      this.socket.emit('task-update', {
        taskId: task.id,
        status: 'in-progress',
        agentId: AGENT_CONFIG.id
      });

      // Generate code based on task requirements
      const result = await this.generateCode(task);

      // Send results back to Agent Manager
      this.socket.emit('task-complete', {
        taskId: task.id,
        agentId: AGENT_CONFIG.id,
        status: 'completed',
        result: result
      });

      logger.info(`Task ${task.id} completed successfully`);
    } catch (error) {
      logger.error(`Error processing task ${task.id}:`, error);
      
      this.socket.emit('task-error', {
        taskId: task.id,
        agentId: AGENT_CONFIG.id,
        status: 'failed',
        error: error.message
      });
    } finally {
      AGENT_CONFIG.status = 'idle';
      this.currentTask = null;
    }
  }

  async generateCode(task) {
    const { type, requirements, template } = task.payload || {};
    
    switch (type) {
      case 'rest-api':
        return this.codeGenerator.generateRestAPI(requirements);
      
      case 'react-component':
        return this.codeGenerator.generateReactComponent(requirements);
      
      case 'express-server':
        return this.codeGenerator.generateExpressServer(requirements);
      
      case 'database-schema':
        return this.codeGenerator.generateDatabaseSchema(requirements);
      
      case 'custom':
        return this.codeGenerator.generateFromPrompt(requirements);
      
      default:
        throw new Error(`Unknown code generation type: ${type}`);
    }
  }

  // Graceful shutdown
  shutdown() {
    logger.info('Shutting down Code Generation Agent');
    
    if (this.socket) {
      this.socket.emit('agent-shutdown', {
        agentId: AGENT_CONFIG.id,
        reason: 'Manual shutdown'
      });
      this.socket.disconnect();
    }
    
    process.exit(0);
  }
}

// Create and start the agent
const agent = new CodeGenAgent();

// Handle process signals
process.on('SIGINT', () => agent.shutdown());
process.on('SIGTERM', () => agent.shutdown());

// Start the agent
(async () => {
  try {
    await agent.connect();
    logger.info('Code Generation Agent started');
  } catch (error) {
    logger.error('Failed to start agent:', error);
    process.exit(1);
  }
})();