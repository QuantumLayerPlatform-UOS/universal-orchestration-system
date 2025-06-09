import { DatabaseManager } from '../config/database';
import { AgentRepository } from '../repositories/agentRepository';
import { AgentRegistryV2 } from './agentRegistryV2';
import { AgentRegistry } from './agentRegistry';
import { MongoDBService } from './mongodbService';
import { logger } from '../utils/logger';

export interface ServiceConfig {
  mongoUri: string;
  dbName: string;
  redisUrl: string;
  useV2Registry?: boolean;
}

export class ServiceInitializer {
  private dbManager: DatabaseManager;
  private repository?: AgentRepository;
  private registryV2?: AgentRegistryV2;
  private legacyRegistry?: AgentRegistry;
  private mongoService?: MongoDBService;

  constructor(private config: ServiceConfig) {
    this.dbManager = new DatabaseManager({
      mongoUri: config.mongoUri,
      dbName: config.dbName,
      redisUrl: config.redisUrl
    });
  }

  async initialize(): Promise<{
    registry: AgentRegistry | AgentRegistryV2;
    mongoService: MongoDBService;
  }> {
    logger.info('Initializing services...');

    // Connect to databases
    await this.dbManager.connect();

    if (this.config.useV2Registry) {
      // Initialize V2 components
      return await this.initializeV2();
    } else {
      // Initialize legacy components
      return await this.initializeLegacy();
    }
  }

  private async initializeV2(): Promise<{
    registry: AgentRegistryV2;
    mongoService: MongoDBService;
  }> {
    logger.info('Initializing V2 registry with distributed support');

    // Create repository
    this.repository = new AgentRepository({
      db: this.dbManager.getDb(),
      redis: this.dbManager.getRedis(),
      cachePrefix: 'agent:',
      cacheTTL: 300
    });
    await this.repository.initialize();

    // Create V2 registry
    this.registryV2 = new AgentRegistryV2({
      repository: this.repository,
      redis: this.dbManager.getRedis(),
      heartbeatTimeout: 30000,
      heartbeatCheckInterval: 10000,
      syncInterval: 5000
    });
    await this.registryV2.initialize();

    // Create MongoDB service for compatibility
    this.mongoService = new MongoDBService();
    this.mongoService.setDb(this.dbManager.getDb());

    return {
      registry: this.registryV2,
      mongoService: this.mongoService
    };
  }

  private async initializeLegacy(): Promise<{
    registry: AgentRegistry;
    mongoService: MongoDBService;
  }> {
    logger.info('Initializing legacy registry');

    // Create MongoDB service
    this.mongoService = new MongoDBService();
    await this.mongoService.connect();

    // Create legacy registry
    this.legacyRegistry = new AgentRegistry(this.mongoService);
    await this.legacyRegistry.initialize();

    return {
      registry: this.legacyRegistry,
      mongoService: this.mongoService
    };
  }

  async cleanup(): Promise<void> {
    logger.info('Cleaning up services...');

    if (this.registryV2) {
      await this.registryV2.cleanup();
    }

    await this.dbManager.disconnect();
  }
}