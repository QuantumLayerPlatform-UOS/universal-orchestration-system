import { Router, Request, Response, NextFunction } from 'express';
import { AgentRegistry } from '../services/agentRegistry';
import { AgentOrchestrator } from '../services/agentOrchestrator';
import { validateRequest, schemas } from '../middleware/validateRequest';
import { ApiError, NotFoundError } from '../middleware/errorHandler';
import { AgentFilter, AgentStatus, AgentType } from '../models/agent';
import Joi from 'joi';

export const agentRoutes = (
  agentRegistry: AgentRegistry,
  agentOrchestrator: AgentOrchestrator
) => {
  const router = Router();

  // Register a new agent
  router.post(
    '/',
    validateRequest({
      body: schemas.agentRegistration
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const agent = await agentRegistry.registerAgent(req.body);
        res.status(201).json({
          message: 'Agent registered successfully',
          agent
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get all agents
  router.get(
    '/',
    validateRequest({
      query: Joi.object({
        type: Joi.string(),
        status: Joi.string(),
        capabilities: Joi.array().items(Joi.string()).single(),
        tags: Joi.array().items(Joi.string()).single(),
        region: Joi.string()
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const filter: AgentFilter = {
          type: req.query.type ? req.query.type as AgentType : undefined,
          status: req.query.status ? req.query.status as AgentStatus : undefined,
          capabilities: Array.isArray(req.query.capabilities) ? req.query.capabilities as string[] : undefined,
          tags: Array.isArray(req.query.tags) ? req.query.tags as string[] : undefined,
          region: req.query.region as string
        };

        const agents = agentRegistry.findAgents(filter);
        
        res.json({
          agents,
          count: agents.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get agent by ID
  router.get(
    '/:agentId',
    validateRequest({
      params: Joi.object({
        agentId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const agent = agentRegistry.getAgent(req.params.agentId);
        
        if (!agent) {
          throw new NotFoundError('Agent');
        }

        res.json({ agent });
      } catch (error) {
        next(error);
      }
    }
  );

  // Update agent status
  router.patch(
    '/:agentId/status',
    validateRequest({
      params: Joi.object({
        agentId: schemas.id
      }),
      body: Joi.object({
        status: Joi.string().valid(...Object.values(AgentStatus)).required()
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await agentRegistry.updateAgentStatus(
          req.params.agentId,
          req.body.status
        );

        res.json({
          message: 'Agent status updated successfully'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Unregister agent
  router.delete(
    '/:agentId',
    validateRequest({
      params: Joi.object({
        agentId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        await agentRegistry.unregisterAgent(req.params.agentId);
        
        res.json({
          message: 'Agent unregistered successfully'
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get agent health check
  router.get(
    '/:agentId/health',
    validateRequest({
      params: Joi.object({
        agentId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        // Health check functionality not yet implemented
        throw new ApiError(501, 'Health check not implemented');
      } catch (error) {
        next(error);
      }
    }
  );

  // Get agent metrics
  router.get(
    '/:agentId/metrics',
    validateRequest({
      params: Joi.object({
        agentId: schemas.id
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const agent = agentRegistry.getAgent(req.params.agentId);
        
        if (!agent) {
          throw new NotFoundError('Agent');
        }

        res.json({
          agentId: agent.id,
          metrics: agent.metrics
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Get available agents by type
  router.get(
    '/available/:type',
    validateRequest({
      params: Joi.object({
        type: Joi.string().required()
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const agents = agentRegistry.getAvailableAgents(req.params.type as AgentType);
        
        res.json({
          type: req.params.type,
          agents,
          count: agents.length
        });
      } catch (error) {
        next(error);
      }
    }
  );

  // Find best agent for task
  router.post(
    '/find-best',
    validateRequest({
      body: Joi.object({
        type: Joi.string().required(),
        requiredCapabilities: Joi.array().items(Joi.string())
      })
    }),
    async (req: Request, res: Response, next: NextFunction) => {
      try {
        const agent = agentRegistry.findBestAgent(
          req.body.type,
          req.body.requiredCapabilities
        );

        if (!agent) {
          res.status(404).json({
            message: 'No suitable agent found',
            criteria: req.body
          });
          return;
        }

        res.json({ agent });
      } catch (error) {
        next(error);
      }
    }
  );

  return router;
};