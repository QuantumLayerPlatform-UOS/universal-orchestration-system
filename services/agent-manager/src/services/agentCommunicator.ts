import { Socket, Server as SocketIOServer } from 'socket.io';
import { v4 as uuidv4 } from 'uuid';
import { EventEmitter } from 'events';
import { 
  Agent, 
  AgentMessage, 
  AgentEvent,
  Task,
  TaskResult,
  AgentStatus
} from '../models/agent';
import { AgentRegistry } from './agentRegistry';
import { logger } from '../utils/logger';

interface PendingMessage {
  message: AgentMessage;
  callback?: (response: any) => void;
  timeout: NodeJS.Timeout;
  retries: number;
  maxRetries: number;
}

export class AgentCommunicator extends EventEmitter {
  private io: SocketIOServer;
  private agentRegistry: AgentRegistry;
  private agentSockets: Map<string, Socket> = new Map();
  private pendingMessages: Map<string, PendingMessage> = new Map();
  private readonly MESSAGE_TIMEOUT = 30000; // 30 seconds
  private readonly MAX_RETRIES = 3;

  constructor(io: SocketIOServer, agentRegistry: AgentRegistry) {
    super();
    this.io = io;
    this.agentRegistry = agentRegistry;
    this.setupOrchestorListeners();
  }

  private setupOrchestorListeners(): void {
    // Listen for task assignments from orchestrator
    this.agentRegistry.on('task:assigned', (task: Task, agent: Agent) => {
      this.sendTaskToAgent(agent.id, task);
    });
  }

  public async handleAgentConnection(socket: Socket): Promise<void> {
    const agentId = socket.data.agentId;
    
    if (!agentId) {
      logger.error('Agent connected without ID');
      socket.disconnect();
      return;
    }

    logger.info(`Handling connection for agent ${agentId}`);

    // Store socket reference
    this.agentSockets.set(agentId, socket);

    // Set up a promise to handle agent registration/verification
    const agentVerificationPromise = new Promise<void>(async (resolve, reject) => {
      try {
        // First, try to get the agent from registry
        const existingAgent = await this.agentRegistry.getAgent(agentId);
        
        if (existingAgent) {
          // Agent exists, update status
          logger.info(`Agent ${agentId} found in registry, updating status`);
          await this.agentRegistry.updateAgentStatus(agentId, AgentStatus.AVAILABLE);
          resolve();
        } else {
          // Agent not found, wait for agent:info event
          logger.info(`Agent ${agentId} not found in registry, waiting for agent:info event`);
          
          // Set up one-time listener for agent:info
          const infoHandler = async (agentInfo: any) => {
            try {
              logger.info(`Received agent:info for ${agentId}`, agentInfo);
              
              // Validate agent info
              if (!agentInfo || !agentInfo.name || !agentInfo.type || !agentInfo.capabilities) {
                throw new Error('Invalid agent info received');
              }
              
              // Register the agent
              await this.agentRegistry.registerAgent(agentInfo, socket.id);
              logger.info(`Agent ${agentId} registered successfully`);
              
              resolve();
            } catch (error) {
              logger.error(`Failed to register agent ${agentId}:`, error);
              reject(error);
            }
          };
          
          socket.once('agent:info', infoHandler);
          
          // Request agent info
          socket.emit('agent:info:request', { timestamp: new Date() });
          
          // Set timeout for agent info
          setTimeout(() => {
            socket.off('agent:info', infoHandler);
            reject(new Error(`Timeout waiting for agent:info from ${agentId}`));
          }, 10000); // 10 second timeout
        }
      } catch (error) {
        reject(error);
      }
    });

    try {
      // Wait for agent verification
      await agentVerificationPromise;
      
      // Setup event handlers
      this.setupAgentEventHandlers(socket, agentId);

      // Send welcome message
      this.sendMessage(agentId, {
        id: uuidv4(),
        from: 'orchestrator',
        to: agentId,
        type: 'event',
        topic: 'welcome',
        payload: {
          message: 'Connected to Agent Manager',
          timestamp: new Date()
        },
        timestamp: new Date()
      });
    } catch (error) {
      logger.error(`Failed to verify agent ${agentId}:`, error);
      this.agentSockets.delete(agentId);
      socket.disconnect();
    }
  }

  public handleAgentDisconnection(socket: Socket): void {
    const agentId = socket.data.agentId;
    
    if (agentId) {
      this.agentSockets.delete(agentId);
      this.agentRegistry.updateAgentStatus(agentId, AgentStatus.OFFLINE);
      
      // Cancel pending messages
      for (const [messageId, pending] of this.pendingMessages) {
        if (pending.message.to === agentId) {
          clearTimeout(pending.timeout);
          if (pending.callback) {
            pending.callback({
              error: 'Agent disconnected'
            });
          }
          this.pendingMessages.delete(messageId);
        }
      }
    }
  }

  private setupAgentEventHandlers(socket: Socket, agentId: string): void {
    // Agent info (for auto-registration)
    socket.on('agent:info', async (agentInfo: any) => {
      logger.info(`Received agent info from ${agentId}`, agentInfo);
      try {
        // Try to register the agent if not already registered
        const existingAgent = await this.agentRegistry.getAgent(agentId);
        if (!existingAgent) {
          logger.info(`Auto-registering agent ${agentId}`);
          await this.agentRegistry.registerAgent(agentInfo, socket.id);
        }
      } catch (error) {
        logger.debug(`Agent ${agentId} auto-registration check:`, error);
      }
    });

    // Heartbeat
    socket.on('heartbeat', async () => {
      await this.agentRegistry.updateAgentHeartbeat(agentId);
      socket.emit('heartbeat:ack', { timestamp: new Date() });
    });

    // Status update
    socket.on('status:update', async (status: AgentStatus) => {
      try {
        await this.agentRegistry.updateAgentStatus(agentId, status);
        socket.emit('status:ack', { status });
      } catch (error) {
        logger.error(`Failed to update agent ${agentId} status`, error);
        socket.emit('error', { message: 'Failed to update status' });
      }
    });

    // Task result
    socket.on('task:result', (result: TaskResult) => {
      logger.info(`Received task result from agent ${agentId} for task ${result.taskId}`);
      this.emit('task:result', result);
    });

    // Task progress
    socket.on('task:progress', (data: { taskId: string; progress: number; message?: string }) => {
      logger.debug(`Task ${data.taskId} progress: ${data.progress}%`);
      this.emit('task:progress', data);
    });

    // Agent events
    socket.on('agent:event', (event: AgentEvent) => {
      logger.info(`Agent event from ${agentId}: ${event.event}`);
      this.emit('agent:event', event);
    });

    // Message handling
    socket.on('message', (message: AgentMessage) => {
      this.handleAgentMessage(agentId, message);
    });

    // Error handling
    socket.on('error', (error: any) => {
      logger.error(`Socket error from agent ${agentId}`, error);
    });
  }

  private async sendTaskToAgent(agentId: string, task: Task): Promise<void> {
    const message: AgentMessage = {
      id: uuidv4(),
      from: 'orchestrator',
      to: agentId,
      type: 'request',
      topic: 'task:execute',
      payload: task,
      correlationId: task.id,
      timestamp: new Date()
    };

    try {
      const response = await this.sendMessageWithResponse(agentId, message);
      
      if (response.error) {
        logger.error(`Agent ${agentId} rejected task ${task.id}:`, response.error);
        this.emit('task:rejected', task, response.error);
      } else {
        logger.info(`Task ${task.id} accepted by agent ${agentId}`);
        this.emit('task:accepted', task);
      }
    } catch (error) {
      logger.error(`Failed to send task ${task.id} to agent ${agentId}`, error);
      this.emit('task:failed', task, error);
    }
  }

  public sendMessage(agentId: string, message: AgentMessage): boolean {
    const socket = this.agentSockets.get(agentId);
    
    if (!socket || !socket.connected) {
      logger.warn(`Cannot send message to agent ${agentId}: Not connected`);
      return false;
    }

    socket.emit('message', message);
    return true;
  }

  public async sendMessageWithResponse(
    agentId: string, 
    message: AgentMessage,
    timeoutMs: number = this.MESSAGE_TIMEOUT
  ): Promise<any> {
    return new Promise((resolve, reject) => {
      const socket = this.agentSockets.get(agentId);
      
      if (!socket || !socket.connected) {
        reject(new Error(`Agent ${agentId} not connected`));
        return;
      }

      const messageId = message.id;
      
      // Setup timeout
      const timeout = setTimeout(() => {
        const pending = this.pendingMessages.get(messageId);
        if (pending) {
          this.pendingMessages.delete(messageId);
          
          if (pending.retries < pending.maxRetries) {
            // Retry
            pending.retries++;
            logger.warn(`Retrying message ${messageId} to agent ${agentId} (attempt ${pending.retries + 1})`);
            
            this.sendMessageWithResponse(agentId, message, timeoutMs)
              .then(resolve)
              .catch(reject);
          } else {
            reject(new Error(`Message ${messageId} timed out after ${pending.maxRetries} retries`));
          }
        }
      }, timeoutMs);

      // Store pending message
      this.pendingMessages.set(messageId, {
        message,
        callback: (response) => {
          clearTimeout(timeout);
          this.pendingMessages.delete(messageId);
          resolve(response);
        },
        timeout,
        retries: 0,
        maxRetries: this.MAX_RETRIES
      });

      // Send message
      socket.emit('message', message);
    });
  }

  public broadcastToAgents(
    filter: (agent: Agent) => boolean, 
    message: Omit<AgentMessage, 'to'>
  ): number {
    const agents = this.agentRegistry.getAllAgents().filter(filter);
    let sent = 0;

    for (const agent of agents) {
      const agentMessage: AgentMessage = {
        ...message,
        to: agent.id
      };

      if (this.sendMessage(agent.id, agentMessage)) {
        sent++;
      }
    }

    logger.info(`Broadcast message to ${sent} agents`);
    return sent;
  }

  public broadcastEvent(event: string, data?: any): void {
    const message: Omit<AgentMessage, 'to'> = {
      id: uuidv4(),
      from: 'orchestrator',
      type: 'broadcast',
      topic: event,
      payload: data,
      timestamp: new Date()
    };

    this.broadcastToAgents(() => true, message);
  }

  private handleAgentMessage(agentId: string, message: AgentMessage): void {
    logger.debug(`Message from agent ${agentId}: ${message.topic}`);

    // Check if this is a response to a pending message
    if (message.correlationId) {
      const pending = this.pendingMessages.get(message.correlationId);
      if (pending && pending.callback) {
        pending.callback(message.payload);
        return;
      }
    }

    // Handle other message types
    switch (message.type) {
      case 'event':
        this.emit(`agent:${message.topic}`, agentId, message.payload);
        break;
      
      case 'request':
        // Agent is requesting something from orchestrator
        this.handleAgentRequest(agentId, message);
        break;
      
      default:
        logger.warn(`Unknown message type from agent ${agentId}: ${message.type}`);
    }
  }

  private async handleAgentRequest(agentId: string, message: AgentMessage): Promise<void> {
    try {
      let response: any;

      switch (message.topic) {
        case 'agent:list':
          response = this.agentRegistry.getAllAgents();
          break;
        
        case 'task:status':
          // Would query task status from task queue
          response = { status: 'not_implemented' };
          break;
        
        default:
          response = { error: `Unknown request topic: ${message.topic}` };
      }

      // Send response
      const responseMessage: AgentMessage = {
        id: uuidv4(),
        from: 'orchestrator',
        to: agentId,
        type: 'response',
        topic: message.topic,
        payload: response,
        correlationId: message.id,
        timestamp: new Date()
      };

      this.sendMessage(agentId, responseMessage);
    } catch (error) {
      logger.error(`Failed to handle agent request from ${agentId}`, error);
    }
  }

  public getConnectedAgents(): string[] {
    return Array.from(this.agentSockets.keys());
  }

  public isAgentConnected(agentId: string): boolean {
    const socket = this.agentSockets.get(agentId);
    return socket?.connected || false;
  }

  public async cleanup(): Promise<void> {
    // Clear pending messages
    for (const [_, pending] of this.pendingMessages) {
      clearTimeout(pending.timeout);
      if (pending.callback) {
        pending.callback({ error: 'Communicator shutting down' });
      }
    }
    
    this.pendingMessages.clear();
    this.agentSockets.clear();
    this.removeAllListeners();
  }
}