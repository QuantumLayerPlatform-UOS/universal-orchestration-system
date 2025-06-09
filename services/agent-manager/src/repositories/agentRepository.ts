import { Collection, Db } from 'mongodb';
import { Agent, AgentStatus } from '../models/agent';
import { logger } from '../utils/logger';
import Redis from 'ioredis';

export interface AgentRepositoryOptions {
  db: Db;
  redis: Redis;
  cachePrefix?: string;
  cacheTTL?: number;
}

export class AgentRepository {
  private collection: Collection<Agent>;
  private redis: Redis;
  private cachePrefix: string;
  private cacheTTL: number;

  constructor(options: AgentRepositoryOptions) {
    this.collection = options.db.collection<Agent>('agents');
    this.redis = options.redis;
    this.cachePrefix = options.cachePrefix || 'agent:';
    this.cacheTTL = options.cacheTTL || 300; // 5 minutes default
  }

  async initialize(): Promise<void> {
    // Create indexes
    await this.collection.createIndex({ id: 1 }, { unique: true });
    await this.collection.createIndex({ type: 1, status: 1 });
    await this.collection.createIndex({ 'metadata.region': 1 });
    await this.collection.createIndex({ updatedAt: -1 });
    
    logger.info('Agent repository initialized with indexes');
  }

  /**
   * Get agent by ID - checks cache first, then database
   */
  async findById(agentId: string): Promise<Agent | null> {
    // Check cache first
    const cached = await this.getFromCache(agentId);
    if (cached) {
      return cached;
    }

    // Load from database
    const agent = await this.collection.findOne({ id: agentId });
    
    if (agent) {
      // Update cache
      await this.setCache(agentId, agent);
    }

    return agent;
  }

  /**
   * Create or update agent
   */
  async upsert(agent: Agent): Promise<Agent> {
    const result = await this.collection.replaceOne(
      { id: agent.id },
      agent,
      { upsert: true }
    );

    // Update cache
    await this.setCache(agent.id, agent);

    // Publish event
    await this.publishEvent('agent:upserted', agent);

    return agent;
  }

  /**
   * Update agent status
   */
  async updateStatus(agentId: string, status: AgentStatus): Promise<Agent | null> {
    const result = await this.collection.findOneAndUpdate(
      { id: agentId },
      { 
        $set: { 
          status,
          updatedAt: new Date(),
          lastHeartbeat: status === AgentStatus.AVAILABLE ? new Date() : undefined
        } 
      },
      { returnDocument: 'after' }
    );

    if (result) {
      // Update cache
      await this.setCache(agentId, result);
      
      // Publish event
      await this.publishEvent('agent:status:changed', {
        agentId,
        status,
        timestamp: new Date()
      });
    }

    return result;
  }

  /**
   * Update agent heartbeat
   */
  async updateHeartbeat(agentId: string): Promise<boolean> {
    const result = await this.collection.updateOne(
      { id: agentId },
      { 
        $set: { 
          lastHeartbeat: new Date(),
          'metrics.lastActive': new Date()
        } 
      }
    );

    if (result.modifiedCount > 0) {
      // Refresh cache
      const agent = await this.findById(agentId);
      if (agent) {
        await this.setCache(agentId, agent);
      }
    }

    return result.modifiedCount > 0;
  }

  /**
   * Find agents by criteria
   */
  async find(criteria: {
    type?: string;
    status?: AgentStatus;
    region?: string;
  }): Promise<Agent[]> {
    const query: any = {};
    
    if (criteria.type) query.type = criteria.type;
    if (criteria.status) query.status = criteria.status;
    if (criteria.region) query['metadata.region'] = criteria.region;

    return await this.collection.find(query).toArray();
  }

  /**
   * Get all active agents
   */
  async findActive(): Promise<Agent[]> {
    return await this.collection.find({
      status: { $ne: AgentStatus.OFFLINE }
    }).toArray();
  }

  /**
   * Mark stale agents as offline
   */
  async markStaleAgentsOffline(threshold: number = 60000): Promise<number> {
    const staleTime = new Date(Date.now() - threshold);
    
    const result = await this.collection.updateMany(
      {
        status: { $ne: AgentStatus.OFFLINE },
        lastHeartbeat: { $lt: staleTime }
      },
      {
        $set: {
          status: AgentStatus.OFFLINE,
          updatedAt: new Date()
        }
      }
    );

    if (result.modifiedCount > 0) {
      logger.info(`Marked ${result.modifiedCount} stale agents as offline`);
      
      // Clear cache for affected agents
      const staleAgents = await this.collection.find({
        status: AgentStatus.OFFLINE,
        updatedAt: { $gte: new Date(Date.now() - 1000) }
      }).toArray();
      
      for (const agent of staleAgents) {
        await this.clearCache(agent.id);
      }
    }

    return result.modifiedCount;
  }

  /**
   * Delete agent
   */
  async delete(agentId: string): Promise<boolean> {
    const result = await this.collection.deleteOne({ id: agentId });
    
    if (result.deletedCount > 0) {
      await this.clearCache(agentId);
      await this.publishEvent('agent:deleted', { agentId });
    }

    return result.deletedCount > 0;
  }

  // Cache operations
  private async getFromCache(agentId: string): Promise<Agent | null> {
    try {
      const data = await this.redis.get(`${this.cachePrefix}${agentId}`);
      return data ? JSON.parse(data) : null;
    } catch (error) {
      logger.error('Cache get error:', error);
      return null;
    }
  }

  private async setCache(agentId: string, agent: Agent): Promise<void> {
    try {
      await this.redis.setex(
        `${this.cachePrefix}${agentId}`,
        this.cacheTTL,
        JSON.stringify(agent)
      );
    } catch (error) {
      logger.error('Cache set error:', error);
    }
  }

  private async clearCache(agentId: string): Promise<void> {
    try {
      await this.redis.del(`${this.cachePrefix}${agentId}`);
    } catch (error) {
      logger.error('Cache clear error:', error);
    }
  }

  // Event publishing
  private async publishEvent(event: string, data: any): Promise<void> {
    try {
      await this.redis.publish(event, JSON.stringify(data));
    } catch (error) {
      logger.error('Event publish error:', error);
    }
  }
}