import { Request, Response, NextFunction } from 'express';
import { v4 as uuidv4 } from 'uuid';
import { logger } from '../utils/logger';

declare global {
  namespace Express {
    interface Request {
      id: string;
      startTime: number;
    }
  }
}

export const requestLogger = (req: Request, res: Response, next: NextFunction): void => {
  // Generate request ID
  req.id = req.headers['x-request-id'] as string || uuidv4();
  req.startTime = Date.now();

  // Log request
  logger.info('Incoming request', {
    requestId: req.id,
    method: req.method,
    path: req.path,
    query: req.query,
    ip: req.ip,
    userAgent: req.get('user-agent')
  });

  // Capture response
  const originalSend = res.send;
  res.send = function (data: any): Response {
    res.send = originalSend;
    
    const duration = Date.now() - req.startTime;
    
    // Log response
    logger.info('Outgoing response', {
      requestId: req.id,
      method: req.method,
      path: req.path,
      statusCode: res.statusCode,
      duration,
      contentLength: res.get('content-length')
    });

    // Log slow requests
    if (duration > 1000) {
      logger.warn('Slow request detected', {
        requestId: req.id,
        path: req.path,
        duration
      });
    }

    return res.send(data);
  };

  // Add request ID to response headers
  res.setHeader('X-Request-ID', req.id);

  next();
};