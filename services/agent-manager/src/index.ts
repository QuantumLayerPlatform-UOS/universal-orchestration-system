import express, { Application } from 'express';
import { createServer } from 'http';
import { Server as SocketIOServer } from 'socket.io';
import cors from 'cors';
import helmet from 'helmet';
import compression from 'compression';
import dotenv from 'dotenv';
import { logger } from './utils/logger';
import { errorHandler } from './middleware/errorHandler';
import { requestLogger } from './middleware/requestLogger';
import { validateRequest } from './middleware/validateRequest';
import { agentRoutes } from './routes/agentRoutes';
import { healthRoutes } from './routes/healthRoutes';
import { taskRoutes } from './routes/taskRoutes';
import { AgentRegistry } from './services/agentRegistry';
import { AgentOrchestrator } from './services/agentOrchestrator';
import { AgentCommunicator } from './services/agentCommunicator';
import { TaskQueue } from './queues/taskQueue';
import { MongoDBService } from './services/mongodbService';
import { MetricsService } from './services/metricsService';
import { gracefulShutdown } from './utils/gracefulShutdown';

// Load environment variables
dotenv.config();

// Validate environment variables
const requiredEnvVars = [
  'PORT',
  'NODE_ENV',
  'MONGODB_URI',
  'REDIS_URL',
  'AZURE_TENANT_ID',
  'AZURE_CLIENT_ID',
  'AZURE_CLIENT_SECRET'
];

for (const envVar of requiredEnvVars) {
  if (!process.env[envVar]) {
    logger.error(`Missing required environment variable: ${envVar}`);
    process.exit(1);
  }
}

const PORT = parseInt(process.env.PORT || '3002', 10);
const NODE_ENV = process.env.NODE_ENV || 'development';

class AgentManagerServer {
  private app: Application;
  private httpServer: any;
  private io: SocketIOServer;
  private agentRegistry: AgentRegistry;
  private agentOrchestrator: AgentOrchestrator;
  private agentCommunicator: AgentCommunicator;
  private taskQueue: TaskQueue;
  private mongoService: MongoDBService;
  private metricsService: MetricsService;

  constructor() {
    this.app = express();
    this.httpServer = createServer(this.app);
    this.io = new SocketIOServer(this.httpServer, {
      cors: {
        origin: process.env.ALLOWED_ORIGINS?.split(',') || '*',
        methods: ['GET', 'POST'],
        credentials: true
      },
      transports: ['websocket', 'polling']
    });

    // Initialize services
    this.mongoService = new MongoDBService();
    this.taskQueue = new TaskQueue();
    this.agentRegistry = new AgentRegistry(this.mongoService);
    this.agentOrchestrator = new AgentOrchestrator(
      this.agentRegistry,
      this.taskQueue
    );
    this.agentCommunicator = new AgentCommunicator(this.io, this.agentRegistry);
    this.metricsService = new MetricsService();
  }

  private setupMiddleware(): void {
    // Security middleware
    this.app.use(helmet({
      contentSecurityPolicy: false // Disable for API
    }));

    // CORS configuration
    this.app.use(cors({
      origin: process.env.ALLOWED_ORIGINS?.split(',') || '*',
      credentials: true,
      methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'],
      allowedHeaders: ['Content-Type', 'Authorization', 'X-Request-ID']
    }));

    // Compression
    this.app.use(compression());

    // Body parsing
    this.app.use(express.json({ limit: '10mb' }));
    this.app.use(express.urlencoded({ extended: true, limit: '10mb' }));

    // Request logging
    this.app.use(requestLogger);

    // Trust proxy
    if (NODE_ENV === 'production') {
      this.app.set('trust proxy', 1);
    }
  }

  private setupRoutes(): void {
    // Health check routes
    this.app.use('/health', healthRoutes);

    // API routes
    const apiRouter = express.Router();

    // Agent management routes
    apiRouter.use('/agents', agentRoutes(
      this.agentRegistry,
      this.agentOrchestrator
    ));

    // Task management routes
    apiRouter.use('/tasks', taskRoutes(
      this.taskQueue,
      this.agentOrchestrator
    ));

    // Metrics endpoint
    apiRouter.get('/metrics', async (req, res) => {
      const metrics = await this.metricsService.getMetrics();
      res.json(metrics);
    });

    // Mount API routes
    this.app.use('/api/v1', apiRouter);

    // 404 handler
    this.app.use((req, res) => {
      res.status(404).json({
        error: 'Not Found',
        message: 'The requested resource does not exist',
        path: req.path
      });
    });

    // Error handler (must be last)
    this.app.use(errorHandler);
  }

  private setupSocketIO(): void {
    // Socket.IO namespace for agent communication
    const agentNamespace = this.io.of('/agents');

    agentNamespace.use((socket, next) => {
      // Authenticate agent connection
      const token = socket.handshake.auth.token;
      if (!token) {
        return next(new Error('Authentication required'));
      }

      // Validate token (implement your auth logic)
      // For now, we'll accept any token
      socket.data.agentId = socket.handshake.auth.agentId;
      next();
    });

    agentNamespace.on('connection', async (socket) => {
      logger.info(`Agent connected: ${socket.data.agentId}`);
      
      // Initialize agent communication handlers
      try {
        await this.agentCommunicator.handleAgentConnection(socket);
      } catch (error) {
        logger.error(`Failed to handle agent connection: ${error}`);
        socket.disconnect();
        return;
      }

      socket.on('disconnect', (reason) => {
        logger.info(`Agent disconnected: ${socket.data.agentId}, reason: ${reason}`);
        this.agentCommunicator.handleAgentDisconnection(socket);
      });
    });

    // Socket.IO namespace for client monitoring
    const monitorNamespace = this.io.of('/monitor');

    monitorNamespace.on('connection', (socket) => {
      logger.info(`Monitor client connected: ${socket.id}`);

      // Send initial state
      socket.emit('agents:state', this.agentRegistry.getAllAgents());

      // Subscribe to agent updates
      const unsubscribe = this.agentRegistry.subscribe((agents) => {
        socket.emit('agents:update', agents);
      });

      socket.on('disconnect', () => {
        logger.info(`Monitor client disconnected: ${socket.id}`);
        unsubscribe();
      });
    });
  }

  public async start(): Promise<void> {
    try {
      // Connect to MongoDB
      await this.mongoService.connect();
      logger.info('Connected to MongoDB');

      // Initialize agent registry
      await this.agentRegistry.initialize();
      logger.info('Agent registry initialized');

      // Initialize task queue
      await this.taskQueue.initialize();
      logger.info('Task queue initialized');

      // Setup middleware and routes
      this.setupMiddleware();
      this.setupRoutes();
      this.setupSocketIO();

      // Start HTTP server
      this.httpServer.listen(PORT, () => {
        logger.info(`Agent Manager service started on port ${PORT}`);
        logger.info(`Environment: ${NODE_ENV}`);
        logger.info(`Process ID: ${process.pid}`);
      });

      // Setup graceful shutdown
      gracefulShutdown(async () => {
        logger.info('Shutting down Agent Manager service...');
        
        // Stop accepting new connections
        this.httpServer.close();
        
        // Close Socket.IO connections
        this.io.close();
        
        // Stop task queue
        await this.taskQueue.close();
        
        // Disconnect from MongoDB
        await this.mongoService.disconnect();
        
        logger.info('Agent Manager service shut down successfully');
      });

    } catch (error) {
      logger.error('Failed to start Agent Manager service', error);
      process.exit(1);
    }
  }
}

// Start the server
const server = new AgentManagerServer();
server.start().catch((error) => {
  logger.error('Unhandled error during startup', error);
  process.exit(1);
});

// Handle unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  logger.error('Unhandled Rejection at:', promise, 'reason:', reason);
  // Application specific logging, throwing an error, or other logic here
});

export default server;