import { MongoClient, Db, Collection } from 'mongodb';
import { logger } from '../utils/logger';

export class MongoDBService {
  private client: MongoClient | null = null;
  private db: Db | null = null;
  private readonly dbName: string;
  private readonly uri: string;

  constructor() {
    this.uri = process.env.MONGODB_URI || 'mongodb://localhost:27017';
    this.dbName = process.env.MONGODB_DB_NAME || 'agent-manager';
  }

  public async connect(): Promise<void> {
    try {
      this.client = new MongoClient(this.uri, {
        maxPoolSize: 10,
        minPoolSize: 2,
        serverSelectionTimeoutMS: 5000,
        socketTimeoutMS: 45000,
      });

      await this.client.connect();
      this.db = this.client.db(this.dbName);
      
      // Create indexes
      await this.createIndexes();
      
      logger.info(`Connected to MongoDB: ${this.dbName}`);
    } catch (error) {
      logger.error('Failed to connect to MongoDB', error);
      throw error;
    }
  }

  private async createIndexes(): Promise<void> {
    if (!this.db) return;

    try {
      // Agent indexes
      const agentsCollection = this.db.collection('agents');
      await agentsCollection.createIndex({ id: 1 }, { unique: true });
      await agentsCollection.createIndex({ type: 1, status: 1 });
      await agentsCollection.createIndex({ lastHeartbeat: 1 });
      await agentsCollection.createIndex({ 'metadata.region': 1 });
      await agentsCollection.createIndex({ 'metadata.tags': 1 });

      // Task indexes
      const tasksCollection = this.db.collection('tasks');
      await tasksCollection.createIndex({ id: 1 }, { unique: true });
      await tasksCollection.createIndex({ status: 1, priority: 1 });
      await tasksCollection.createIndex({ type: 1, status: 1 });
      await tasksCollection.createIndex({ assignedAgentId: 1 });
      await tasksCollection.createIndex({ createdAt: -1 });
      await tasksCollection.createIndex({ 'metadata.userId': 1 });
      await tasksCollection.createIndex({ 'metadata.projectId': 1 });

      // TTL index for cleanup of old completed tasks
      await tasksCollection.createIndex(
        { completedAt: 1 },
        { 
          expireAfterSeconds: 7 * 24 * 60 * 60, // 7 days
          partialFilterExpression: { status: 'completed' }
        }
      );

      logger.info('MongoDB indexes created');
    } catch (error) {
      logger.error('Failed to create MongoDB indexes', error);
    }
  }

  public getCollection<T>(name: string): Collection<T> {
    if (!this.db) {
      throw new Error('MongoDB not connected');
    }
    return this.db.collection<T>(name);
  }

  public async disconnect(): Promise<void> {
    if (this.client) {
      await this.client.close();
      this.client = null;
      this.db = null;
      logger.info('Disconnected from MongoDB');
    }
  }

  public isConnected(): boolean {
    return this.client !== null && this.client.topology?.isConnected() || false;
  }

  public async ping(): Promise<boolean> {
    if (!this.db) return false;
    
    try {
      await this.db.admin().ping();
      return true;
    } catch (error) {
      return false;
    }
  }

  public async getStats(): Promise<any> {
    if (!this.db) {
      throw new Error('MongoDB not connected');
    }

    const [agents, tasks] = await Promise.all([
      this.db.collection('agents').countDocuments(),
      this.db.collection('tasks').countDocuments()
    ]);

    return {
      connected: this.isConnected(),
      database: this.dbName,
      collections: {
        agents,
        tasks
      }
    };
  }
}