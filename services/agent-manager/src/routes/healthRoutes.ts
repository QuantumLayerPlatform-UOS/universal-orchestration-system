import { Router, Request, Response } from 'express';
import { MongoDBService } from '../services/mongodbService';
import { TaskQueue } from '../queues/taskQueue';
import os from 'os';

export const healthRoutes = Router();

let mongoService: MongoDBService;
let taskQueue: TaskQueue;

// Initialize dependencies
export const initHealthRoutes = (mongo: MongoDBService, queue: TaskQueue) => {
  mongoService = mongo;
  taskQueue = queue;
};

// Basic health check
healthRoutes.get('/', async (req: Request, res: Response) => {
  res.json({
    status: 'healthy',
    service: 'agent-manager',
    timestamp: new Date().toISOString()
  });
});

// Detailed health check
healthRoutes.get('/detailed', async (req: Request, res: Response) => {
  try {
    const checks = {
      service: 'agent-manager',
      status: 'healthy',
      timestamp: new Date().toISOString(),
      version: process.env.npm_package_version || '1.0.0',
      uptime: process.uptime(),
      environment: process.env.NODE_ENV || 'development',
      checks: {
        mongodb: await checkMongoDB(),
        redis: await checkRedis(),
        memory: checkMemory(),
        cpu: checkCPU()
      }
    };

    const hasFailure = Object.values(checks.checks).some(check => check.status === 'unhealthy');
    checks.status = hasFailure ? 'degraded' : 'healthy';

    res.status(hasFailure ? 503 : 200).json(checks);
  } catch (error: any) {
    res.status(503).json({
      status: 'unhealthy',
      error: error.message,
      timestamp: new Date().toISOString()
    });
  }
});

// Liveness probe
healthRoutes.get('/live', (req: Request, res: Response) => {
  res.status(200).send('OK');
});

// Readiness probe
healthRoutes.get('/ready', async (req: Request, res: Response) => {
  try {
    // Check if MongoDB is connected
    if (!mongoService || !mongoService.isConnected()) {
      throw new Error('MongoDB not connected');
    }

    // Check if Redis/Queue is available
    const queueStats = await taskQueue.getQueueStats();
    if (!queueStats) {
      throw new Error('Task queue not available');
    }

    res.status(200).json({
      status: 'ready',
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    res.status(503).json({
      status: 'not ready',
      error: error.message,
      timestamp: new Date().toISOString()
    });
  }
});

// Helper functions
async function checkMongoDB(): Promise<any> {
  try {
    if (!mongoService) {
      return { status: 'unhealthy', message: 'MongoDB service not initialized' };
    }

    const isConnected = mongoService.isConnected();
    const canPing = isConnected ? await mongoService.ping() : false;

    return {
      status: isConnected && canPing ? 'healthy' : 'unhealthy',
      connected: isConnected,
      responsive: canPing
    };
  } catch (error: any) {
    return {
      status: 'unhealthy',
      error: error.message
    };
  }
}

async function checkRedis(): Promise<any> {
  try {
    if (!taskQueue) {
      return { status: 'unhealthy', message: 'Task queue not initialized' };
    }

    const stats = await taskQueue.getQueueStats();
    
    return {
      status: 'healthy',
      stats
    };
  } catch (error: any) {
    return {
      status: 'unhealthy',
      error: error.message
    };
  }
}

function checkMemory(): any {
  const totalMemory = os.totalmem();
  const freeMemory = os.freemem();
  const usedMemory = totalMemory - freeMemory;
  const memoryUsagePercent = (usedMemory / totalMemory) * 100;

  return {
    status: memoryUsagePercent < 90 ? 'healthy' : 'unhealthy',
    usage: {
      total: totalMemory,
      free: freeMemory,
      used: usedMemory,
      percentage: memoryUsagePercent.toFixed(2)
    },
    process: {
      heapTotal: process.memoryUsage().heapTotal,
      heapUsed: process.memoryUsage().heapUsed,
      rss: process.memoryUsage().rss
    }
  };
}

function checkCPU(): any {
  const cpus = os.cpus();
  const loadAverage = os.loadavg();
  
  return {
    status: loadAverage[0] < cpus.length * 0.8 ? 'healthy' : 'unhealthy',
    cores: cpus.length,
    loadAverage: {
      '1min': loadAverage[0],
      '5min': loadAverage[1],
      '15min': loadAverage[2]
    }
  };
}