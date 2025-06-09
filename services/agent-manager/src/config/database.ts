import { MongoClient, Db } from 'mongodb';
import Redis from 'ioredis';
import { logger } from '../utils/logger';

export interface DatabaseConfig {
  mongoUri: string;
  dbName: string;
  redisUrl: string;
}

export class DatabaseManager {
  private mongoClient: MongoClient | null = null;
  private db: Db | null = null;
  private redis: Redis | null = null;
  private config: DatabaseConfig;

  constructor(config: DatabaseConfig) {
    this.config = config;
  }

  async connect(): Promise<void> {
    try {
      // Connect to MongoDB
      this.mongoClient = new MongoClient(this.config.mongoUri);
      await this.mongoClient.connect();
      this.db = this.mongoClient.db(this.config.dbName);
      logger.info('Connected to MongoDB');

      // Connect to Redis
      this.redis = new Redis(this.config.redisUrl);
      
      this.redis.on('connect', () => {
        logger.info('Connected to Redis');
      });

      this.redis.on('error', (error) => {
        logger.error('Redis connection error:', error);
      });

      // Test Redis connection
      await this.redis.ping();
      
    } catch (error) {
      logger.error('Database connection failed:', error);
      throw error;
    }
  }

  getDb(): Db {
    if (!this.db) {
      throw new Error('Database not connected');
    }
    return this.db;
  }

  getRedis(): Redis {
    if (!this.redis) {
      throw new Error('Redis not connected');
    }
    return this.redis;
  }

  async disconnect(): Promise<void> {
    try {
      if (this.mongoClient) {
        await this.mongoClient.close();
        logger.info('Disconnected from MongoDB');
      }

      if (this.redis) {
        this.redis.disconnect();
        logger.info('Disconnected from Redis');
      }
    } catch (error) {
      logger.error('Error during database disconnect:', error);
    }
  }
}