import { v4 as uuidv4 } from 'uuid';
import pLimit from 'p-limit';
import pRetry from 'p-retry';
import { EventEmitter } from 'events';
import { 
  Task, 
  TaskStatus, 
  TaskRequest, 
  TaskAssignment, 
  TaskResult,
  AgentType,
  TaskPriority
} from '../models/agent';
import { AgentRegistry } from './agentRegistry';
import { TaskQueue } from '../queues/taskQueue';
import { logger } from '../utils/logger';

interface OrchestrationStrategy {
  name: string;
  canHandle(task: Task): boolean;
  orchestrate(task: Task, orchestrator: AgentOrchestrator): Promise<TaskResult>;
}

export class AgentOrchestrator extends EventEmitter {
  private agentRegistry: AgentRegistry;
  private taskQueue: TaskQueue;
  private activeAssignments: Map<string, TaskAssignment> = new Map();
  private strategies: Map<string, OrchestrationStrategy> = new Map();
  private concurrencyLimit = pLimit(10); // Limit concurrent orchestrations
  private readonly ASSIGNMENT_TIMEOUT = 300000; // 5 minutes
  private assignmentCheckInterval: NodeJS.Timeout | null = null;

  constructor(agentRegistry: AgentRegistry, taskQueue: TaskQueue) {
    super();
    this.agentRegistry = agentRegistry;
    this.taskQueue = taskQueue;
    this.initializeStrategies();
    this.startAssignmentMonitor();
    this.setupTaskQueueHandlers();
  }

  private initializeStrategies(): void {
    // Simple single-agent strategy
    this.strategies.set('single-agent', {
      name: 'single-agent',
      canHandle: (task: Task) => true, // Default strategy
      orchestrate: async (task: Task, orchestrator: AgentOrchestrator) => {
        const agent = orchestrator.agentRegistry.findBestAgent(
          task.type,
          task.requiredCapabilities
        );

        if (!agent) {
          throw new Error(`No available agent found for task type: ${task.type}`);
        }

        return orchestrator.assignTaskToAgent(task, agent.id);
      }
    });

    // Multi-agent pipeline strategy
    this.strategies.set('pipeline', {
      name: 'pipeline',
      canHandle: (task: Task) => {
        return task.metadata.tags?.includes('pipeline') || false;
      },
      orchestrate: async (task: Task, orchestrator: AgentOrchestrator) => {
        const pipeline = task.payload.pipeline as Array<{
          type: AgentType;
          capabilities?: string[];
        }>;

        if (!pipeline || !Array.isArray(pipeline)) {
          throw new Error('Pipeline configuration missing or invalid');
        }

        let previousResult: any = task.payload.initialData;
        let finalResult: TaskResult | null = null;

        for (let i = 0; i < pipeline.length; i++) {
          const stage = pipeline[i];
          const stageTask: Task = {
            ...task,
            id: `${task.id}-stage-${i}`,
            type: stage.type,
            requiredCapabilities: stage.capabilities,
            payload: {
              ...task.payload,
              stageIndex: i,
              previousResult,
              isLastStage: i === pipeline.length - 1
            }
          };

          const result = await orchestrator.orchestrateTask(stageTask);
          
          if (result.status === 'failure') {
            return result; // Pipeline failed
          }

          previousResult = result.result;
          finalResult = result;
        }

        return finalResult!;
      }
    });

    // Parallel execution strategy
    this.strategies.set('parallel', {
      name: 'parallel',
      canHandle: (task: Task) => {
        return task.metadata.tags?.includes('parallel') || false;
      },
      orchestrate: async (task: Task, orchestrator: AgentOrchestrator) => {
        const subtasks = task.payload.subtasks as Array<{
          type: AgentType;
          payload: any;
          capabilities?: string[];
        }>;

        if (!subtasks || !Array.isArray(subtasks)) {
          throw new Error('Subtasks configuration missing or invalid');
        }

        const results = await Promise.all(
          subtasks.map(async (subtask, index) => {
            const subTask: Task = {
              ...task,
              id: `${task.id}-sub-${index}`,
              type: subtask.type,
              requiredCapabilities: subtask.capabilities,
              payload: subtask.payload
            };

            return orchestrator.orchestrateTask(subTask);
          })
        );

        const hasFailure = results.some(r => r.status === 'failure');
        
        return {
          taskId: task.id,
          agentId: 'orchestrator',
          status: hasFailure ? 'failure' : 'success',
          result: results,
          error: hasFailure ? {
            code: 'PARALLEL_EXECUTION_FAILURE',
            message: 'One or more subtasks failed'
          } : undefined
        };
      }
    });
  }

  private setupTaskQueueHandlers(): void {
    this.taskQueue.on('task:ready', async (task: Task) => {
      try {
        await this.orchestrateTask(task);
      } catch (error) {
        logger.error(`Failed to orchestrate task ${task.id}`, error);
        await this.handleTaskFailure(task, error);
      }
    });
  }

  private startAssignmentMonitor(): void {
    this.assignmentCheckInterval = setInterval(() => {
      this.checkExpiredAssignments();
    }, 30000); // Check every 30 seconds
  }

  private checkExpiredAssignments(): void {
    const now = new Date();
    const expiredAssignments: string[] = [];

    for (const [taskId, assignment] of this.activeAssignments) {
      if (assignment.expiresAt < now) {
        expiredAssignments.push(taskId);
      }
    }

    for (const taskId of expiredAssignments) {
      logger.warn(`Task assignment expired: ${taskId}`);
      this.handleAssignmentTimeout(taskId);
    }
  }

  private mapPriorityToNumber(priority?: string): TaskPriority {
    switch (priority?.toLowerCase()) {
      case 'critical':
        return TaskPriority.CRITICAL;
      case 'high':
        return TaskPriority.HIGH;
      case 'medium':
        return TaskPriority.MEDIUM;
      case 'low':
        return TaskPriority.LOW;
      default:
        return TaskPriority.MEDIUM;
    }
  }

  public async submitTask(request: TaskRequest): Promise<Task> {
    const task: Task = {
      id: uuidv4(),
      type: request.type,
      priority: typeof request.priority === 'string' 
        ? this.mapPriorityToNumber(request.priority) 
        : (request.priority ?? TaskPriority.MEDIUM),
      status: TaskStatus.PENDING,
      payload: request.payload,
      requiredCapabilities: request.requiredCapabilities,
      metadata: {
        source: request.metadata?.source || 'api',
        ...request.metadata
      },
      attempts: 0,
      maxAttempts: request.maxAttempts || 3,
      timeout: request.timeout || 300000, // 5 minutes default
      createdAt: new Date(),
      updatedAt: new Date()
    };

    // Add to queue
    await this.taskQueue.addTask(task);
    
    logger.info(`Task submitted: ${task.id} (type: ${task.type})`);
    this.emit('task:submitted', task);

    return task;
  }

  public async orchestrateTask(task: Task): Promise<TaskResult> {
    return this.concurrencyLimit(async () => {
      try {
        // Update task status
        task.status = TaskStatus.ASSIGNED;
        task.updatedAt = new Date();
        await this.taskQueue.updateTask(task);

        // Find appropriate strategy
        const strategy = this.findStrategy(task);
        
        logger.info(`Orchestrating task ${task.id} with strategy: ${strategy.name}`);
        
        // Execute with retry logic
        const result = await pRetry(
          async () => {
            return await strategy.orchestrate(task, this);
          },
          {
            retries: task.maxAttempts - 1,
            onFailedAttempt: (error) => {
              logger.warn(`Task ${task.id} attempt ${error.attemptNumber} failed`, error);
              task.attempts = error.attemptNumber;
            }
          }
        );

        // Update task with result
        task.status = result.status === 'success' ? TaskStatus.COMPLETED : TaskStatus.FAILED;
        task.result = result.result;
        task.error = result.error?.message;
        task.completedAt = new Date();
        task.updatedAt = new Date();
        await this.taskQueue.updateTask(task);

        this.emit('task:completed', task, result);
        return result;

      } catch (error: any) {
        await this.handleTaskFailure(task, error);
        throw error;
      }
    });
  }

  private findStrategy(task: Task): OrchestrationStrategy {
    for (const [_, strategy] of this.strategies) {
      if (strategy.canHandle(task)) {
        return strategy;
      }
    }
    
    // Return default single-agent strategy
    return this.strategies.get('single-agent')!;
  }

  private async assignTaskToAgent(task: Task, agentId: string): Promise<TaskResult> {
    const agent = this.agentRegistry.getAgent(agentId);
    if (!agent) {
      throw new Error(`Agent ${agentId} not found`);
    }

    // Create assignment
    const assignment: TaskAssignment = {
      taskId: task.id,
      agentId: agentId,
      assignedAt: new Date(),
      expiresAt: new Date(Date.now() + this.ASSIGNMENT_TIMEOUT)
    };

    this.activeAssignments.set(task.id, assignment);

    // Update task
    task.assignedAgentId = agentId;
    task.assignedAt = assignment.assignedAt;
    task.status = TaskStatus.IN_PROGRESS;
    await this.taskQueue.updateTask(task);

    // Emit assignment event (agent communicator will handle sending to agent)
    this.emit('task:assigned', task, agent);

    // Wait for result with timeout
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.removeAllListeners(`task:${task.id}:result`);
        reject(new Error(`Task ${task.id} timed out`));
      }, task.timeout);

      this.once(`task:${task.id}:result`, (result: TaskResult) => {
        clearTimeout(timeout);
        this.activeAssignments.delete(task.id);
        resolve(result);
      });
    });
  }

  public async handleTaskResult(taskId: string, result: TaskResult): Promise<void> {
    const assignment = this.activeAssignments.get(taskId);
    if (!assignment) {
      logger.warn(`Received result for unassigned task: ${taskId}`);
      return;
    }

    // Update agent metrics
    const agent = this.agentRegistry.getAgent(assignment.agentId);
    if (agent) {
      const responseTime = result.metrics?.duration || 0;
      const currentMetrics = agent.metrics;
      
      if (result.status === 'success') {
        currentMetrics.tasksCompleted++;
      } else {
        currentMetrics.tasksFailed++;
      }
      
      // Update average response time
      const totalTasks = currentMetrics.tasksCompleted + currentMetrics.tasksFailed;
      currentMetrics.averageResponseTime = 
        (currentMetrics.averageResponseTime * (totalTasks - 1) + responseTime) / totalTasks;
      
      await this.agentRegistry.updateAgentMetrics(assignment.agentId, currentMetrics);
    }

    // Emit result event
    this.emit(`task:${taskId}:result`, result);
  }

  private async handleTaskFailure(task: Task, error: any): Promise<void> {
    logger.error(`Task ${task.id} failed`, error);
    
    task.status = TaskStatus.FAILED;
    task.error = error.message || 'Unknown error';
    task.completedAt = new Date();
    task.updatedAt = new Date();
    
    await this.taskQueue.updateTask(task);
    
    this.emit('task:failed', task, error);
  }

  private async handleAssignmentTimeout(taskId: string): Promise<void> {
    const assignment = this.activeAssignments.get(taskId);
    if (!assignment) return;

    this.activeAssignments.delete(taskId);
    
    // Get task from queue
    const task = await this.taskQueue.getTask(taskId);
    if (!task) return;

    // Retry or fail based on attempts
    if (task.attempts < task.maxAttempts) {
      task.status = TaskStatus.RETRYING;
      task.attempts++;
      await this.taskQueue.updateTask(task);
      
      // Re-queue for retry
      await this.taskQueue.requeueTask(task);
    } else {
      await this.handleTaskFailure(task, new Error('Task assignment timeout'));
    }
  }

  public async aggregateResults(taskIds: string[]): Promise<any> {
    const results = await Promise.all(
      taskIds.map(async (taskId) => {
        const task = await this.taskQueue.getTask(taskId);
        return task?.result;
      })
    );

    return results.filter(r => r !== null && r !== undefined);
  }

  public getActiveAssignments(): TaskAssignment[] {
    return Array.from(this.activeAssignments.values());
  }

  public async cleanup(): Promise<void> {
    if (this.assignmentCheckInterval) {
      clearInterval(this.assignmentCheckInterval);
      this.assignmentCheckInterval = null;
    }
    
    this.activeAssignments.clear();
    this.removeAllListeners();
  }
}