import { EventEmitter } from 'events';
import { Agent, AgentStatus, AgentType, AgentFilter, AgentRegistrationRequest } from '../models/agent';
import { AgentRepository } from '../repositories/agentRepository';
import { logger } from '../utils/logger';
import { v4 as uuidv4 } from 'uuid';
import Redis from 'ioredis';

export interface AgentRegistryOptions {
  repository: AgentRepository;
  redis: Redis;
  heartbeatTimeout?: number;
  heartbeatCheckInterval?: number;
  syncInterval?: number;
}

export class AgentRegistry extends EventEmitter {
  private repository: AgentRepository;
  private redis: Redis;
  private localCache: Map<string, Agent> = new Map();
  private heartbeatTimeout: number;
  private heartbeatCheckInterval: number;
  private syncInterval: number;
  private intervals: NodeJS.Timeout[] = [];
  private subscribers: ((agents: Agent[]) => void)[] = [];

  constructor(options: AgentRegistryOptions) {
    super();
    this.repository = options.repository;
    this.redis = options.redis;
    this.heartbeatTimeout = options.heartbeatTimeout || 30000;
    this.heartbeatCheckInterval = options.heartbeatCheckInterval || 10000;
    this.syncInterval = options.syncInterval || 5000;
  }

  async initialize(): Promise<void> {
    logger.info('Initializing AgentRegistry with distributed support');

    // Load all active agents into local cache
    await this.syncFromDatabase();

    // Start background tasks
    this.startBackgroundTasks();

    // Subscribe to Redis events for real-time updates
    this.subscribeToEvents();

    logger.info('AgentRegistry initialized');
  }

  private async syncFromDatabase(): Promise<void> {
    try {
      const agents = await this.repository.findActive();
      
      // Clear and rebuild local cache
      this.localCache.clear();
      for (const agent of agents) {
        this.localCache.set(agent.id, agent);
      }

      logger.info(`Synced ${agents.length} agents from database`);
      this.notifySubscribers();
    } catch (error) {
      logger.error('Failed to sync from database:', error);
    }
  }

  private startBackgroundTasks(): void {
    // Heartbeat monitoring
    const heartbeatInterval = setInterval(() => {
      this.checkHeartbeats();
    }, this.heartbeatCheckInterval);
    this.intervals.push(heartbeatInterval);

    // Database sync
    const syncInterval = setInterval(() => {
      this.syncFromDatabase();
    }, this.syncInterval);
    this.intervals.push(syncInterval);

    // Stale agent cleanup
    const cleanupInterval = setInterval(() => {
      this.repository.markStaleAgentsOffline(this.heartbeatTimeout * 2);
    }, 60000); // Every minute
    this.intervals.push(cleanupInterval);
  }

  private subscribeToEvents(): void {
    const subscriber = this.redis.duplicate();
    
    subscriber.on('message', (channel, message) => {
      try {
        const data = JSON.parse(message);
        this.handleRedisEvent(channel, data);
      } catch (error) {
        logger.error('Failed to handle Redis event:', error);
      }
    });

    subscriber.subscribe(
      'agent:upserted',
      'agent:status:changed',
      'agent:deleted'
    );
  }

  private handleRedisEvent(channel: string, data: any): void {
    switch (channel) {
      case 'agent:upserted':
        this.localCache.set(data.id, data);
        this.emit('agent:registered', data);
        this.notifySubscribers();
        break;

      case 'agent:status:changed':
        const agent = this.localCache.get(data.agentId);
        if (agent) {
          agent.status = data.status;
          agent.updatedAt = new Date(data.timestamp);
          this.emit('agent:status:updated', agent);
          this.notifySubscribers();
        }
        break;

      case 'agent:deleted':
        this.localCache.delete(data.agentId);
        this.emit('agent:unregistered', { id: data.agentId });
        this.notifySubscribers();
        break;
    }
  }

  /**
   * Register a new agent or update existing
   */
  async registerAgent(request: AgentRegistrationRequest, socketId?: string): Promise<Agent> {
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

    // Save to repository (handles both DB and cache)
    const savedAgent = await this.repository.upsert(agent);
    
    // Update local cache
    this.localCache.set(agentId, savedAgent);

    logger.info(`Agent registered: ${agentId}`);
    this.emit('agent:registered', savedAgent);
    this.notifySubscribers();

    return savedAgent;
  }

  /**
   * Update agent status
   */
  async updateAgentStatus(agentId: string, status: AgentStatus): Promise<void> {
    // First check local cache
    let agent = this.localCache.get(agentId);
    
    if (!agent) {
      // Try to load from repository
      agent = await this.repository.findById(agentId) || undefined;
      
      if (!agent) {
        throw new Error(`Agent ${agentId} not found`);
      }
      
      // Add to local cache
      this.localCache.set(agentId, agent);
    }

    // Update in repository
    const updatedAgent = await this.repository.updateStatus(agentId, status);
    
    if (updatedAgent) {
      // Update local cache
      this.localCache.set(agentId, updatedAgent);
      this.emit('agent:status:updated', updatedAgent);
      this.notifySubscribers();
    }
  }

  /**
   * Update agent heartbeat
   */
  async updateAgentHeartbeat(agentId: string): Promise<void> {
    const success = await this.repository.updateHeartbeat(agentId);
    
    if (success) {
      // Update local cache timestamp
      const agent = this.localCache.get(agentId);
      if (agent) {
        agent.lastHeartbeat = new Date();
        agent.metrics.lastActive = new Date();
      }
    } else {
      // Agent not found in DB, try to reload
      const agent = await this.repository.findById(agentId);
      if (agent) {
        this.localCache.set(agentId, agent);
        await this.repository.updateHeartbeat(agentId);
      } else {
        throw new Error(`Agent ${agentId} not found`);
      }
    }
  }

  /**
   * Get agent by ID (synchronous - returns from cache only)
   */
  getAgent(agentId: string): Agent | undefined {
    return this.localCache.get(agentId);
  }

  /**
   * Get agent by ID with repository fallback
   */
  async getAgentAsync(agentId: string): Promise<Agent | undefined> {
    // Check local cache first
    let agent = this.localCache.get(agentId);
    
    if (!agent) {
      // Try to load from repository
      const dbAgent = await this.repository.findById(agentId);
      if (dbAgent) {
        this.localCache.set(agentId, dbAgent);
        agent = dbAgent;
      }
    }
    
    return agent;
  }

  /**
   * Get all agents
   */
  getAllAgents(): Agent[] {
    return Array.from(this.localCache.values());
  }

  /**
   * Get available agents by type
   */
  getAvailableAgents(type?: AgentType): Agent[] {
    return Array.from(this.localCache.values()).filter(agent => {
      const isAvailable = agent.status === AgentStatus.AVAILABLE;
      const matchesType = !type || agent.type === type;
      return isAvailable && matchesType;
    });
  }

  /**
   * Unregister agent
   */
  async unregisterAgent(agentId: string): Promise<void> {
    const success = await this.repository.delete(agentId);
    
    if (success) {
      this.localCache.delete(agentId);
      this.emit('agent:unregistered', { id: agentId });
      this.notifySubscribers();
    }
  }

  /**
   * Check heartbeats and mark stale agents offline
   */
  private async checkHeartbeats(): Promise<void> {
    const now = Date.now();
    const updates: Promise<any>[] = [];

    for (const [agentId, agent] of this.localCache) {
      const timeSinceLastHeartbeat = now - agent.lastHeartbeat.getTime();
      
      if (agent.status !== AgentStatus.OFFLINE && timeSinceLastHeartbeat > this.heartbeatTimeout) {
        logger.warn(`Agent ${agentId} missed heartbeat, marking as offline`);
        updates.push(this.updateAgentStatus(agentId, AgentStatus.OFFLINE));
      }
    }

    await Promise.all(updates);
  }

  /**
   * Subscribe to agent updates
   */
  subscribe(callback: (agents: Agent[]) => void): () => void {
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
        logger.error('Subscriber callback error:', error);
      }
    });
  }

  /**
   * Update agent metrics
   */
  async updateAgentMetrics(agentId: string, metrics: Partial<Agent['metrics']>): Promise<void> {
    const agent = await this.getAgentAsync(agentId);
    if (!agent) {
      throw new Error(`Agent ${agentId} not found`);
    }

    agent.metrics = { ...agent.metrics, ...metrics };
    agent.updatedAt = new Date();

    // Update in database using collection directly
    const collection = this.repository['collection'];
    await collection.updateOne(
      { id: agentId },
      { 
        $set: { 
          metrics: agent.metrics,
          updatedAt: agent.updatedAt
        } 
      }
    );

    // Update local cache
    this.localCache.set(agentId, agent);
    this.notifySubscribers();
  }

  /**
   * Find agents matching a filter
   */
  findAgents(filter: AgentFilter): Agent[] {
    return this.getAllAgents().filter(agent => {
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

  /**
   * Find the best agent for a task
   */
  findBestAgent(type: AgentType, requiredCapabilities?: string[]): Agent | null {
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
      
      // Then by fewer failed tasks
      const failedDiff = a.metrics.tasksFailed - b.metrics.tasksFailed;
      if (failedDiff !== 0) return failedDiff;
      
      // Finally by more completed tasks (experience)
      return b.metrics.tasksCompleted - a.metrics.tasksCompleted;
    });

    return eligibleAgents[0];
  }

  /**
   * Cleanup resources
   */
  async cleanup(): Promise<void> {
    // Clear intervals
    this.intervals.forEach(interval => clearInterval(interval));
    this.intervals = [];

    // Clear subscribers
    this.subscribers = [];

    // Clear cache
    this.localCache.clear();

    logger.info('AgentRegistry cleaned up');
  }
}