import { Router, Request, Response, NextFunction } from 'express';
import { TaskQueue } from '../queues/taskQueue';
import { AgentOrchestrator } from '../services/agentOrchestrator';
import { validateRequest, schemas } from '../middleware/validateRequest';
import { NotFoundError } from '../middleware/errorHandler';
import { TaskStatus, TaskPriority } from '../models/agent';
import Joi from 'joi';

export const taskRoutes = (
  taskQueue: TaskQueue,
  agentOrchestrator: AgentOrchestrator
) => {
  const router = Router();

  // Submit a new task
  router.post(
    '/',
    validateRequest({
      body: schemas.taskSubmission
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const task = await agentOrchestrator.submitTask(req.body);
        
        res.status(201).json({
          message: 'Task submitted successfully',
          task
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get task by ID
  router.get(
    '/:taskId',
    validateRequest({
      params: Joi.object({
        taskId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const task = await taskQueue.getTask(req.params.taskId);
        
        if (!task) {
          throw new NotFoundError('Task');
        }

        res.json({ task });
      } catch (error) {
        next(error);
      }
    }
  );

  // Cancel a task
  router.delete(
    '/:taskId',
    validateRequest({
      params: Joi.object({
        taskId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await taskQueue.cancelTask(req.params.taskId);
        
        res.json({
          message: 'Task cancelled successfully'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get tasks by status
  router.get(
    '/status/:status',
    validateRequest({
      params: Joi.object({
        status: Joi.string().valid(...Object.values(TaskStatus)).required()
      }),
      query: Joi.object({
        limit: Joi.number().integer().min(1).max(100).default(20)
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const tasks = await taskQueue.getTasksByStatus(
          req.params.status as TaskStatus,
          req.query.limit as number
        );

        res.json({
          status: req.params.status,
          tasks,
          count: tasks.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get tasks by priority
  router.get(
    '/priority/:priority',
    validateRequest({
      params: Joi.object({
        priority: Joi.number().valid(...Object.values(TaskPriority).filter(v => typeof v === 'number')).required()
      }),
      query: Joi.object({
        limit: Joi.number().integer().min(1).max(100).default(20)
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const tasks = await taskQueue.getTasksByPriority(
          parseInt(req.params.priority) as TaskPriority,
          req.query.limit as number
        );

        res.json({
          priority: req.params.priority,
          tasks,
          count: tasks.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get queue statistics
  router.get(
    '/queue/stats',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const stats = await taskQueue.getQueueStats();
        
        res.json({
          queue: 'agent-tasks',
          stats,
          timestamp: new Date().toISOString()
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get active assignments
  router.get(
    '/assignments/active',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const assignments = agentOrchestrator.getActiveAssignments();
        
        res.json({
          assignments,
          count: assignments.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Clear completed tasks
  router.post(
    '/queue/clear-completed',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await taskQueue.clearCompleted();
        
        res.json({
          message: 'Completed tasks cleared successfully'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Clear failed tasks
  router.post(
    '/queue/clear-failed',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await taskQueue.clearFailed();
        
        res.json({
          message: 'Failed tasks cleared successfully'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Pause queue processing
  router.post(
    '/queue/pause',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await taskQueue.pause();
        
        res.json({
          message: 'Queue processing paused'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Resume queue processing
  router.post(
    '/queue/resume',
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await taskQueue.resume();
        
        res.json({
          message: 'Queue processing resumed'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Aggregate results for multiple tasks
  router.post(
    '/aggregate',
    validateRequest({
      body: Joi.object({
        taskIds: Joi.array().items(schemas.id).min(1).required()
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const results = await agentOrchestrator.aggregateResults(req.body.taskIds);
        
        res.json({
          taskIds: req.body.taskIds,
          results,
          count: results.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  return router;
};