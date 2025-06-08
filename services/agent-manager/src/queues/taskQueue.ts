import Bull from 'bull';
import { EventEmitter } from 'events';
import { Task, TaskStatus, TaskPriority } from '../models/agent';
import { logger } from '../utils/logger';
import { MongoDBService } from '../services/mongodbService';

interface QueuedTask {
  task: Task;
  retryCount: number;
}

export class TaskQueue extends EventEmitter {
  private queue: Bull.Queue<QueuedTask>;
  private mongoService?: MongoDBService;
  private readonly QUEUE_NAME = 'agent-tasks';
  private readonly REDIS_URL = process.env.REDIS_URL || 'redis://localhost:6379';

  constructor(mongoService?: MongoDBService) {
    super();
    this.mongoService = mongoService;
    
    // Initialize Bull queue
    this.queue = new Bull(this.QUEUE_NAME, this.REDIS_URL, {
      defaultJobOptions: {
        removeOnComplete: 100, // Keep last 100 completed jobs
        removeOnFail: 50, // Keep last 50 failed jobs
        attempts: 3,
        backoff: {
          type: 'exponential',
          delay: 5000 // Start with 5 second delay
        }
      }
    });

    this.setupQueueHandlers();
  }

  private setupQueueHandlers(): void {
    // Process tasks
    this.queue.process(async (job) => {
      const { task } = job.data;
      logger.info(`Processing task ${task.id} from queue`);
      
      // Update task status
      task.status = TaskStatus.ASSIGNED;
      task.updatedAt = new Date();
      
      if (this.mongoService) {
        await this.updateTaskInDatabase(task);
      }
      
      // Emit event for orchestrator to handle
      this.emit('task:ready', task);
      
      // The actual processing is handled by the orchestrator
      // We just mark the job as completed here
      return { taskId: task.id, status: 'orchestrated' };
    });

    // Queue event handlers
    this.queue.on('completed', (job, result) => {
      logger.info(`Task ${job.data.task.id} processing completed`, result);
    });

    this.queue.on('failed', (job, err) => {
      logger.error(`Task ${job.data.task.id} processing failed`, err);
      this.emit('task:queue:failed', job.data.task, err);
    });

    this.queue.on('stalled', (job) => {
      logger.warn(`Task ${job.data.task.id} stalled`);
    });

    this.queue.on('error', (error) => {
      logger.error('Queue error:', error);
    });

    this.queue.on('waiting', (jobId) => {
      logger.debug(`Job ${jobId} is waiting`);
    });

    this.queue.on('active', (job) => {
      logger.debug(`Job ${job.id} has started`);
    });
  }

  public async initialize(): Promise<void> {
    // Clean up any stalled jobs on startup
    await this.queue.clean(0, 'wait');
    await this.queue.clean(0, 'failed');
    
    logger.info('Task queue initialized');
  }

  public async addTask(task: Task): Promise<Bull.Job<QueuedTask>> {
    // Store task in database if available
    if (this.mongoService) {
      await this.storeTaskInDatabase(task);
    }

    // Add to queue with priority
    const job = await this.queue.add(
      {
        task,
        retryCount: 0
      },
      {
        priority: task.priority,
        delay: 0,
        attempts: task.maxAttempts,
        timeout: task.timeout,
        jobId: task.id // Use task ID as job ID for easy lookup
      }
    );

    logger.info(`Task ${task.id} added to queue with priority ${task.priority}`);
    this.emit('task:queued', task);

    return job;
  }

  public async requeueTask(task: Task): Promise<Bull.Job<QueuedTask>> {
    task.status = TaskStatus.RETRYING;
    task.updatedAt = new Date();

    return this.addTask(task);
  }

  public async getTask(taskId: string): Promise<Task | null> {
    // First try to get from queue
    const job = await this.queue.getJob(taskId);
    if (job) {
      return job.data.task;
    }

    // If not in queue, try database
    if (this.mongoService) {
      const collection = this.mongoService.getCollection<Task>('tasks');
      const task = await collection.findOne({ id: taskId });
      return task;
    }

    return null;
  }

  public async updateTask(task: Task): Promise<void> {
    // Update in queue if job exists
    const job = await this.queue.getJob(task.id);
    if (job) {
      await job.update({
        ...job.data,
        task
      });
    }

    // Update in database
    if (this.mongoService) {
      await this.updateTaskInDatabase(task);
    }
  }

  public async cancelTask(taskId: string): Promise<void> {
    const job = await this.queue.getJob(taskId);
    if (job) {
      await job.remove();
      logger.info(`Task ${taskId} cancelled`);
    }

    // Update status in database
    if (this.mongoService) {
      const collection = this.mongoService.getCollection<Task>('tasks');
      await collection.updateOne(
        { id: taskId },
        { 
          $set: { 
            status: TaskStatus.CANCELLED,
            updatedAt: new Date()
          } 
        }
      );
    }

    this.emit('task:cancelled', taskId);
  }

  public async getQueueStats(): Promise<{
    waiting: number;
    active: number;
    completed: number;
    failed: number;
    delayed: number;
  }> {
    const [waiting, active, completed, failed, delayed] = await Promise.all([
      this.queue.getWaitingCount(),
      this.queue.getActiveCount(),
      this.queue.getCompletedCount(),
      this.queue.getFailedCount(),
      this.queue.getDelayedCount()
    ]);

    return { waiting, active, completed, failed, delayed };
  }

  public async getTasksByStatus(status: TaskStatus, limit: number = 100): Promise<Task[]> {
    if (this.mongoService) {
      const collection = this.mongoService.getCollection<Task>('tasks');
      const tasks = await collection
        .find({ status })
        .sort({ createdAt: -1 })
        .limit(limit)
        .toArray();
      return tasks;
    }
    
    return [];
  }

  public async getTasksByPriority(priority: TaskPriority, limit: number = 100): Promise<Task[]> {
    if (this.mongoService) {
      const collection = this.mongoService.getCollection<Task>('tasks');
      const tasks = await collection
        .find({ priority, status: TaskStatus.PENDING })
        .sort({ createdAt: -1 })
        .limit(limit)
        .toArray();
      return tasks;
    }
    
    return [];
  }

  public async clearCompleted(): Promise<void> {
    await this.queue.clean(0, 'completed');
    logger.info('Cleared completed tasks from queue');
  }

  public async clearFailed(): Promise<void> {
    await this.queue.clean(0, 'failed');
    logger.info('Cleared failed tasks from queue');
  }

  private async storeTaskInDatabase(task: Task): Promise<void> {
    if (!this.mongoService) return;

    try {
      const collection = this.mongoService.getCollection<Task>('tasks');
      await collection.insertOne(task);
    } catch (error) {
      logger.error(`Failed to store task ${task.id} in database`, error);
    }
  }

  private async updateTaskInDatabase(task: Task): Promise<void> {
    if (!this.mongoService) return;

    try {
      const collection = this.mongoService.getCollection<Task>('tasks');
      await collection.updateOne(
        { id: task.id },
        { $set: task },
        { upsert: true }
      );
    } catch (error) {
      logger.error(`Failed to update task ${task.id} in database`, error);
    }
  }

  public async close(): Promise<void> {
    await this.queue.close();
    logger.info('Task queue closed');
  }

  public async pause(): Promise<void> {
    await this.queue.pause();
    logger.info('Task queue paused');
  }

  public async resume(): Promise<void> {
    await this.queue.resume();
    logger.info('Task queue resumed');
  }

  public async drain(): Promise<void> {
    await this.queue.empty();
    logger.info('Task queue drained');
  }
}