import { EventEmitter } from 'events';
import { logger } from '../utils/logger';

interface ServiceMetrics {
  uptime: number;
  timestamp: Date;
  agents: {
    total: number;
    byStatus: Record<string, number>;
    byType: Record<string, number>;
  };
  tasks: {
    total: number;
    byStatus: Record<string, number>;
    byPriority: Record<string, number>;
    averageProcessingTime: number;
    successRate: number;
  };
  system: {
    cpuUsage: NodeJS.CpuUsage;
    memoryUsage: NodeJS.MemoryUsage;
    eventLoopDelay: number;
  };
  throughput: {
    tasksPerMinute: number;
    tasksPerHour: number;
    tasksPerDay: number;
  };
}

export class MetricsService extends EventEmitter {
  private metrics: Map<string, any> = new Map();
  private startTime: Date;
  private taskMetrics: {
    completed: number;
    failed: number;
    totalProcessingTime: number;
  };
  private metricsInterval: NodeJS.Timeout | null = null;

  constructor() {
    super();
    this.startTime = new Date();
    this.taskMetrics = {
      completed: 0,
      failed: 0,
      totalProcessingTime: 0
    };
    this.initializeMetricsCollection();
  }

  private initializeMetricsCollection(): void {
    // Collect metrics every minute
    this.metricsInterval = setInterval(() => {
      this.collectSystemMetrics();
    }, 60000);

    // Initial collection
    this.collectSystemMetrics();
  }

  private collectSystemMetrics(): void {
    try {
      const cpuUsage = process.cpuUsage();
      const memoryUsage = process.memoryUsage();
      
      this.metrics.set('system', {
        cpuUsage,
        memoryUsage,
        eventLoopDelay: this.measureEventLoopDelay(),
        timestamp: new Date()
      });

      this.emit('metrics:collected', this.metrics);
    } catch (error) {
      logger.error('Failed to collect system metrics', error);
    }
  }

  private measureEventLoopDelay(): number {
    const start = process.hrtime.bigint();
    setImmediate(() => {
      const delay = Number(process.hrtime.bigint() - start) / 1e6; // Convert to milliseconds
      this.metrics.set('eventLoopDelay', delay);
    });
    return this.metrics.get('eventLoopDelay') || 0;
  }

  public recordTaskCompletion(duration: number, success: boolean): void {
    if (success) {
      this.taskMetrics.completed++;
    } else {
      this.taskMetrics.failed++;
    }
    this.taskMetrics.totalProcessingTime += duration;
  }

  public incrementCounter(name: string, value: number = 1): void {
    const current = this.metrics.get(name) || 0;
    this.metrics.set(name, current + value);
  }

  public setGauge(name: string, value: number): void {
    this.metrics.set(name, value);
  }

  public recordHistogram(name: string, value: number): void {
    const histogram = this.metrics.get(name) || {
      count: 0,
      sum: 0,
      min: Infinity,
      max: -Infinity,
      values: []
    };

    histogram.count++;
    histogram.sum += value;
    histogram.min = Math.min(histogram.min, value);
    histogram.max = Math.max(histogram.max, value);
    histogram.values.push(value);

    // Keep only last 1000 values to prevent memory issues
    if (histogram.values.length > 1000) {
      histogram.values.shift();
    }

    this.metrics.set(name, histogram);
  }

  public async getMetrics(): Promise<ServiceMetrics> {
    const uptime = Date.now() - this.startTime.getTime();
    const totalTasks = this.taskMetrics.completed + this.taskMetrics.failed;
    const successRate = totalTasks > 0 ? this.taskMetrics.completed / totalTasks : 0;
    const averageProcessingTime = totalTasks > 0 ? this.taskMetrics.totalProcessingTime / totalTasks : 0;

    // Calculate throughput
    const uptimeMinutes = uptime / 60000;
    const uptimeHours = uptimeMinutes / 60;
    const uptimeDays = uptimeHours / 24;

    return {
      uptime,
      timestamp: new Date(),
      agents: {
        total: this.metrics.get('agents.total') || 0,
        byStatus: this.metrics.get('agents.byStatus') || {},
        byType: this.metrics.get('agents.byType') || {}
      },
      tasks: {
        total: totalTasks,
        byStatus: this.metrics.get('tasks.byStatus') || {},
        byPriority: this.metrics.get('tasks.byPriority') || {},
        averageProcessingTime,
        successRate
      },
      system: this.metrics.get('system') || {
        cpuUsage: process.cpuUsage(),
        memoryUsage: process.memoryUsage(),
        eventLoopDelay: 0
      },
      throughput: {
        tasksPerMinute: uptimeMinutes > 0 ? totalTasks / uptimeMinutes : 0,
        tasksPerHour: uptimeHours > 0 ? totalTasks / uptimeHours : 0,
        tasksPerDay: uptimeDays > 0 ? totalTasks / uptimeDays : 0
      }
    };
  }

  public reset(): void {
    this.metrics.clear();
    this.taskMetrics = {
      completed: 0,
      failed: 0,
      totalProcessingTime: 0
    };
    this.startTime = new Date();
  }

  public cleanup(): void {
    if (this.metricsInterval) {
      clearInterval(this.metricsInterval);
      this.metricsInterval = null;
    }
    this.removeAllListeners();
  }
}