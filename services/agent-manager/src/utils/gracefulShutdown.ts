import { logger } from './logger';

export const gracefulShutdown = (cleanup: () => Promise<void>): void => {
  let isShuttingDown = false;

  const shutdown = async (signal: string) => {
    if (isShuttingDown) {
      logger.warn('Shutdown already in progress');
      return;
    }

    isShuttingDown = true;
    logger.info(`Received ${signal} signal, starting graceful shutdown`);

    // Set a timeout for graceful shutdown
    const shutdownTimeout = setTimeout(() => {
      logger.error('Graceful shutdown timeout, forcing exit');
      process.exit(1);
    }, 30000); // 30 seconds timeout

    try {
      await cleanup();
      clearTimeout(shutdownTimeout);
      logger.info('Graceful shutdown completed');
      process.exit(0);
    } catch (error) {
      clearTimeout(shutdownTimeout);
      logger.error('Error during graceful shutdown', error);
      process.exit(1);
    }
  };

  // Handle termination signals
  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));
  
  // Handle uncaught exceptions
  process.on('uncaughtException', (error) => {
    logger.error('Uncaught exception', error);
    shutdown('uncaughtException');
  });

  // Handle unhandled promise rejections
  process.on('unhandledRejection', (reason, promise) => {
    logger.error('Unhandled rejection', { reason, promise });
    shutdown('unhandledRejection');
  });
};