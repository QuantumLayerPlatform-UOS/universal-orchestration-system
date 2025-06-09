import { v4 as uuidv4 } from 'uuid';
import { Agent, AgentStatus, AgentType, AgentFilter, AgentRegistrationRequest, AgentHealthCheck } from '../models/agent';
import { MongoDBService } from './mongodbService';
import { logger } from '../utils/logger';
import { EventEmitter } from 'events';

export class AgentRegistry extends EventEmitter {
  private agents: Map<string, Agent> = new Map();
  private mongoService: MongoDBService;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private readonly HEARTBEAT_TIMEOUT = 30000; // 30 seconds
  private readonly HEARTBEAT_CHECK_INTERVAL = 10000; // 10 seconds
  private subscribers: ((agents: Agent[]) => void)[] = [];

  constructor(mongoService: MongoDBService) {
    super();
    this.mongoService = mongoService;
    this.initializeHeartbeatMonitor();
  }

  public async initialize(): Promise<void> {
    await this.loadAgentsFromDatabase();
  }

  private async loadAgentsFromDatabase(): Promise<void> {
    try {
      const collection = this.mongoService.getCollection<Agent>('agents');
      const agents = await collection.find({ status: { $ne: AgentStatus.OFFLINE } }).toArray();
      
      for (const agent of agents) {
        // Reset status to offline until they reconnect
        agent.status = AgentStatus.OFFLINE;
        this.agents.set(agent.id, agent);
      }
      
      logger.info(`Loaded ${agents.length} agents from database`);
    } catch (error) {
      logger.error('Failed to load agents from database', error);
    }
  }

  private initializeHeartbeatMonitor(): void {
    this.heartbeatInterval = setInterval(() => {
      this.checkAgentHeartbeats();
    }, this.HEARTBEAT_CHECK_INTERVAL);
  }

  private checkAgentHeartbeats(): void {
    const now = new Date();
    const updates: Agent[] = [];

    for (const [agentId, agent] of this.agents) {
      const timeSinceLastHeartbeat = now.getTime() - agent.lastHeartbeat.getTime();
      
      if (agent.status !== AgentStatus.OFFLINE && timeSinceLastHeartbeat > this.HEARTBEAT_TIMEOUT) {
        logger.warn(`Agent ${agentId} missed heartbeat, marking as offline`);
        agent.status = AgentStatus.OFFLINE;
        agent.updatedAt = now;
        updates.push(agent);
        this.emit('agent:offline', agent);
      }
    }

    if (updates.length > 0) {
      this.updateAgentsInDatabase(updates);
      this.notifySubscribers();
    }
  }

  public async registerAgent(request: AgentRegistrationRequest, socketId?: string): Promise<Agent> {
    // Use provided ID if available, otherwise generate one
    const agentId = request.id || uuidv4();
    const now = new Date();

    const agent: Agent = {
      id: agentId,
      name: request.name,
      type: request.type,
      status: AgentStatus.AVAILABLE,
      capabilities: request.capabilities,
      endpoint: request.endpoint,
      socketId,
      metadata: request.metadata,
      metrics: {
        tasksCompleted: 0,
        tasksFailed: 0,
        averageResponseTime: 0,
        uptime: 0,
        lastActive: now
      },
      lastHeartbeat: now,
      registeredAt: now,
      updatedAt: now
    };

    // Store in memory
    this.agents.set(agentId, agent);

    // Store in database
    try {
      const collection = this.mongoService.getCollection<Agent>('agents');
      await collection.insertOne(agent);
      logger.info(`Agent registered: ${agentId} (${agent.name})`);
    } catch (error) {
      logger.error(`Failed to store agent ${agentId} in database`, error);
      this.agents.delete(agentId);
      throw error;
    }

    this.emit('agent:registered', agent);
    this.notifySubscribers();

    return agent;
  }

  public async unregisterAgent(agentId: string): Promise<void> {
    const agent = this.agents.get(agentId);
    if (!agent) {
      throw new Error(`Agent ${agentId} not found`);
    }

    // Remove from memory
    this.agents.delete(agentId);

    // Update in database (soft delete - mark as offline)
    try {
      const collection = this.mongoService.getCollection<Agent>('agents');
      await collection.updateOne(
        { id: agentId },
        { 
          $set: { 
            status: AgentStatus.OFFLINE,
            updatedAt: new Date()
          } 
        }
      );
      logger.info(`Agent unregistered: ${agentId}`);
    } catch (error) {
      logger.error(`Failed to update agent ${agentId} in database`, error);
    }

    this.emit('agent:unregistered', agent);
    this.notifySubscribers();
  }

  public async updateAgentStatus(agentId: string, status: AgentStatus): Promise<void> {
    let agent = this.agents.get(agentId);
    
    if (!agent) {
      // Try to load from database
      const collection = this.mongoService.getCollection<Agent>('agents');
      const dbAgent = await collection.findOne({ id: agentId });
      
      if (dbAgent) {
        logger.info(`Loading agent ${agentId} from database into registry`);
        this.agents.set(agentId, dbAgent);
        agent = dbAgent;
      } else {
        throw new Error(`Agent ${agentId} not found`);
      }
    }

    agent.status = status;
    agent.updatedAt = new Date();

    // Update in database
    await this.updateAgentInDatabase(agent);

    this.emit('agent:status:updated', agent);
    this.notifySubscribers();
  }

  public async updateAgentHeartbeat(agentId: string): Promise<void> {
    let agent = this.agents.get(agentId);
    
    if (!agent) {
      // Try to load from database
      const collection = this.mongoService.getCollection<Agent>('agents');
      const dbAgent = await collection.findOne({ id: agentId });
      
      if (dbAgent) {
        logger.info(`Loading agent ${agentId} from database for heartbeat`);
        this.agents.set(agentId, dbAgent);
        agent = dbAgent;
      } else {
        throw new Error(`Agent ${agentId} not found`);
      }
    }

    const now = new Date();
    agent.lastHeartbeat = now;
    agent.metrics.lastActive = now;

    // If agent was offline, mark as available
    if (agent.status === AgentStatus.OFFLINE) {
      agent.status = AgentStatus.AVAILABLE;
      await this.updateAgentInDatabase(agent);
      this.emit('agent:online', agent);
      this.notifySubscribers();
    }
  }

  public async updateAgentMetrics(agentId: string, metrics: Partial<Agent['metrics']>): Promise<void> {
    const agent = this.agents.get(agentId);
    if (!agent) {
      throw new Error(`Agent ${agentId} not found`);
    }

    agent.metrics = { ...agent.metrics, ...metrics };
    agent.updatedAt = new Date();

    await this.updateAgentInDatabase(agent);
  }

  public getAgent(agentId: string): Agent | undefined {
    return this.agents.get(agentId);
  }

  public getAllAgents(): Agent[] {
    return Array.from(this.agents.values());
  }

  public getAvailableAgents(type?: AgentType): Agent[] {
    return Array.from(this.agents.values()).filter(agent => {
      const isAvailable = agent.status === AgentStatus.AVAILABLE;
      const matchesType = !type || agent.type === type;
      return isAvailable && matchesType;
    });
  }

  public findAgents(filter: AgentFilter): Agent[] {
    return Array.from(this.agents.values()).filter(agent => {
      if (filter.type && agent.type !== filter.type) return false;
      if (filter.status && agent.status !== filter.status) return false;
      if (filter.region && agent.metadata.region !== filter.region) return false;
      
      if (filter.capabilities && filter.capabilities.length > 0) {
        const agentCapabilities = agent.capabilities.map(c => c.name);
        const hasAllCapabilities = filter.capabilities.every(cap => 
          agentCapabilities.includes(cap)
        );
        if (!hasAllCapabilities) return false;
      }
      
      if (filter.tags && filter.tags.length > 0) {
        const agentTags = agent.metadata.tags || [];
        const hasAllTags = filter.tags.every(tag => agentTags.includes(tag));
        if (!hasAllTags) return false;
      }
      
      return true;
    });
  }

  public findBestAgent(type: AgentType, requiredCapabilities?: string[]): Agent | null {
    const candidates = this.getAvailableAgents(type);
    
    if (candidates.length === 0) {
      return null;
    }

    // Filter by required capabilities
    let eligibleAgents = candidates;
    if (requiredCapabilities && requiredCapabilities.length > 0) {
      eligibleAgents = candidates.filter(agent => {
        const agentCapabilities = agent.capabilities.map(c => c.name);
        return requiredCapabilities.every(cap => agentCapabilities.includes(cap));
      });
    }

    if (eligibleAgents.length === 0) {
      return null;
    }

    // Sort by metrics (least busy, best performance)
    eligibleAgents.sort((a, b) => {
      // Prefer agents with lower average response time
      const responseTimeDiff = a.metrics.averageResponseTime - b.metrics.averageResponseTime;
      if (responseTimeDiff !== 0) return responseTimeDiff;
      
      // Then prefer agents with higher success rate
      const aSuccessRate = a.metrics.tasksCompleted / (a.metrics.tasksCompleted + a.metrics.tasksFailed || 1);
      const bSuccessRate = b.metrics.tasksCompleted / (b.metrics.tasksCompleted + b.metrics.tasksFailed || 1);
      return bSuccessRate - aSuccessRate;
    });

    return eligibleAgents[0];
  }

  public async performHealthCheck(agentId: string): Promise<AgentHealthCheck> {
    const agent = this.agents.get(agentId);
    if (!agent) {
      throw new Error(`Agent ${agentId} not found`);
    }

    const now = new Date();
    const timeSinceLastHeartbeat = now.getTime() - agent.lastHeartbeat.getTime();
    
    const healthCheck: AgentHealthCheck = {
      agentId,
      status: 'healthy',
      checks: {
        connectivity: timeSinceLastHeartbeat < this.HEARTBEAT_TIMEOUT,
        resources: true, // Would check CPU/memory if available
        capabilities: agent.capabilities.length > 0
      },
      timestamp: now
    };

    if (!healthCheck.checks.connectivity) {
      healthCheck.status = 'unhealthy';
    } else if (!healthCheck.checks.capabilities) {
      healthCheck.status = 'degraded';
    }

    return healthCheck;
  }

  public subscribe(callback: (agents: Agent[]) => void): () => void {
    this.subscribers.push(callback);
    
    // Return unsubscribe function
    return () => {
      const index = this.subscribers.indexOf(callback);
      if (index > -1) {
        this.subscribers.splice(index, 1);
      }
    };
  }

  private notifySubscribers(): void {
    const agents = this.getAllAgents();
    this.subscribers.forEach(callback => {
      try {
        callback(agents);
      } catch (error) {
        logger.error('Error in agent registry subscriber', error);
      }
    });
  }

  private async updateAgentInDatabase(agent: Agent): Promise<void> {
    try {
      const collection = this.mongoService.getCollection<Agent>('agents');
      await collection.updateOne(
        { id: agent.id },
        { $set: agent },
        { upsert: true }
      );
    } catch (error) {
      logger.error(`Failed to update agent ${agent.id} in database`, error);
    }
  }

  private async updateAgentsInDatabase(agents: Agent[]): Promise<void> {
    try {
      const collection = this.mongoService.getCollection<Agent>('agents');
      const bulkOps = agents.map(agent => ({
        updateOne: {
          filter: { id: agent.id },
          update: { $set: agent },
          upsert: true
        }
      }));
      
      await collection.bulkWrite(bulkOps);
    } catch (error) {
      logger.error('Failed to update agents in database', error);
    }
  }

  public async cleanup(): Promise<void> {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
    
    // Mark all agents as offline
    const updates = Array.from(this.agents.values()).map(agent => {
      agent.status = AgentStatus.OFFLINE;
      agent.updatedAt = new Date();
      return agent;
    });
    
    if (updates.length > 0) {
      await this.updateAgentsInDatabase(updates);
    }
    
    this.agents.clear();
    this.subscribers = [];
    this.removeAllListeners();
  }
}