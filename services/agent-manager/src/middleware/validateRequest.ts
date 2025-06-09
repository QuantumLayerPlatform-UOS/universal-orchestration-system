import { Request, Response, NextFunction } from 'express';
import Joi from 'joi';
import { ValidationError } from './errorHandler';

export const validateRequest = (schema: {
  body?: Joi.Schema;
  query?: Joi.Schema;
  params?: Joi.Schema;
}) => {
  return (req: Request, res: Response, next: NextFunction): void => {
    const validationOptions = {
      abortEarly: false, // Include all errors
      allowUnknown: true, // Ignore unknown keys
      stripUnknown: true // Remove unknown keys
    };

    try {
      // Validate body
      if (schema.body) {
        const { error, value } = schema.body.validate(req.body, validationOptions);
        if (error) {
          throw new ValidationError('Invalid request body', formatJoiErrors(error));
        }
        req.body = value;
      }

      // Validate query
      if (schema.query) {
        const { error, value } = schema.query.validate(req.query, validationOptions);
        if (error) {
          throw new ValidationError('Invalid query parameters', formatJoiErrors(error));
        }
        req.query = value;
      }

      // Validate params
      if (schema.params) {
        const { error, value } = schema.params.validate(req.params, validationOptions);
        if (error) {
          throw new ValidationError('Invalid path parameters', formatJoiErrors(error));
        }
        req.params = value;
      }

      next();
    } catch (error) {
      next(error);
    }
  };
};

const formatJoiErrors = (error: Joi.ValidationError): any => {
  return error.details.reduce((acc, detail) => {
    const path = detail.path.join('.');
    acc[path] = detail.message;
    return acc;
  }, {} as Record<string, string>);
};

// Common validation schemas
export const schemas = {
  // ID parameter
  id: Joi.string().uuid().required(),

  // Pagination
  pagination: Joi.object({
    page: Joi.number().integer().min(1).default(1),
    limit: Joi.number().integer().min(1).max(100).default(20),
    sort: Joi.string(),
    order: Joi.string().valid('asc', 'desc').default('desc')
  }),

  // Date range
  dateRange: Joi.object({
    start: Joi.date().iso(),
    end: Joi.date().iso().min(Joi.ref('start'))
  }),

  // Agent registration
  agentRegistration: Joi.object({
    id: Joi.string().optional(),
    name: Joi.string().min(3).max(100).required(),
    type: Joi.string().valid(
      'code-gen', 'test-gen', 'deploy', 'monitor', 
      'security', 'documentation', 'review', 'optimization',
      'meta-prompt', 'dynamic'
    ).required(),
    capabilities: Joi.array().items(
      Joi.object({
        name: Joi.string().required(),
        description: Joi.string().optional(),
        version: Joi.string().required(),
        parameters: Joi.object()
      })
    ).min(1).required(),
    endpoint: Joi.string().uri().optional(),
    metadata: Joi.object({
      version: Joi.string().required(),
      platform: Joi.string().required(),
      region: Joi.string(),
      tags: Joi.array().items(Joi.string())
    }).required()
  }),

  // Task submission
  taskSubmission: Joi.object({
    type: Joi.string().valid(
      'code-gen', 'test-gen', 'deploy', 'monitor', 
      'security', 'documentation', 'review', 'optimization',
      'meta-prompt', 'dynamic'
    ).required(),
    priority: Joi.string().valid('critical', 'high', 'medium', 'low').optional(),
    payload: Joi.object().required(),
    requiredCapabilities: Joi.array().items(Joi.string()),
    metadata: Joi.object({
      source: Joi.string(),
      userId: Joi.string(),
      projectId: Joi.string(),
      correlationId: Joi.string(),
      tags: Joi.array().items(Joi.string())
    }),
    timeout: Joi.number().integer().min(1000).max(3600000), // 1s to 1hr
    maxAttempts: Joi.number().integer().min(1).max(10)
  })
};