import winston from 'winston';
import path from 'path';

const { combine, timestamp, printf, colorize, errors } = winston.format;

// Custom log format
const logFormat = printf(({ level, message, timestamp, stack, ...metadata }) => {
  let log = `${timestamp} [${level}]: ${message}`;
  
  // Add metadata if present
  if (Object.keys(metadata).length > 0) {
    log += ` ${JSON.stringify(metadata)}`;
  }
  
  // Add stack trace for errors
  if (stack) {
    log += `\n${stack}`;
  }
  
  return log;
});

// Create logger instance
export const logger = winston.createLogger({
  level: process.env.LOG_LEVEL || 'info',
  format: combine(
    errors({ stack: true }),
    timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
    logFormat
  ),
  defaultMeta: { service: 'agent-manager' },
  transports: [
    // Console transport for development
    new winston.transports.Console({
      format: combine(
        colorize(),
        timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
        logFormat
      )
    })
  ]
});

// Add file transport for production
if (process.env.NODE_ENV === 'production') {
  const logsDir = process.env.LOGS_DIR || path.join(__dirname, '../../logs');
  
  logger.add(new winston.transports.File({
    filename: path.join(logsDir, 'error.log'),
    level: 'error',
    maxsize: 10485760, // 10MB
    maxFiles: 5,
    tailable: true
  }));
  
  logger.add(new winston.transports.File({
    filename: path.join(logsDir, 'combined.log'),
    maxsize: 10485760, // 10MB
    maxFiles: 10,
    tailable: true
  }));
  
  // Add daily rotate file transport if needed
  // Requires winston-daily-rotate-file package
}

// Stream for Morgan HTTP logging
export const stream = {
  write: (message: string) => {
    logger.info(message.trim());
  }
};

// Helper functions for structured logging
export const logError = (message: string, error: Error, metadata?: any) => {
  logger.error(message, { error: error.message, stack: error.stack, ...metadata });
};

export const logInfo = (message: string, metadata?: any) => {
  logger.info(message, metadata);
};

export const logDebug = (message: string, metadata?: any) => {
  logger.debug(message, metadata);
};

export const logWarn = (message: string, metadata?: any) => {
  logger.warn(message, metadata);
};

// Performance logging
export const logPerformance = (operation: string, duration: number, metadata?: any) => {
  logger.info(`Performance: ${operation}`, { duration_ms: duration, ...metadata });
};

// API request logging
export const logApiRequest = (method: string, path: string, statusCode: number, duration: number, metadata?: any) => {
  const level = statusCode >= 400 ? 'error' : 'info';
  logger.log(level, `API Request: ${method} ${path}`, {
    method,
    path,
    statusCode,
    duration_ms: duration,
    ...metadata
  });
};

export default logger;