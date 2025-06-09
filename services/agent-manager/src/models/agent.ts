import { ObjectId, Document } from 'mongodb';

export enum AgentType {
  CODE_GEN = 'code-gen',
  TEST_GEN = 'test-gen',
  DEPLOY = 'deploy',
  MONITOR = 'monitor',
  SECURITY = 'security',
  DOCUMENTATION = 'documentation',
  REVIEW = 'review',
  OPTIMIZATION = 'optimization',
  META_PROMPT = 'meta-prompt',
  DYNAMIC = 'dynamic'
}

export enum AgentStatus {
  AVAILABLE = 'available',
  BUSY = 'busy',
  OFFLINE = 'offline',
  ERROR = 'error',
  MAINTENANCE = 'maintenance'
}

export enum TaskStatus {
  PENDING = 'pending',
  ASSIGNED = 'assigned',
  IN_PROGRESS = 'in_progress',
  COMPLETED = 'completed',
  FAILED = 'failed',
  CANCELLED = 'cancelled',
  RETRYING = 'retrying'
}

export enum TaskPriority {
  CRITICAL = 0,
  HIGH = 1,
  MEDIUM = 2,
  LOW = 3
}

export interface AgentCapability {
  name: string;
  description?: string;
  version: string;
  parameters?: Record<string, any>;
}

export interface AgentMetrics {
  tasksCompleted: number;
  tasksFailed: number;
  averageResponseTime: number;
  uptime: number;
  lastActive: Date;
  cpuUsage?: number;
  memoryUsage?: number;
}

export interface Agent extends Document {
  _id?: ObjectId;
  id: string;
  name: string;
  type: AgentType;
  status: AgentStatus;
  capabilities: AgentCapability[];
  endpoint?: string;
  socketId?: string;
  metadata: {
    version: string;
    platform: string;
    region?: string;
    tags?: string[];
    isDynamic?: boolean;
    ttl?: number;
    spawnedAt?: string;
    designVersion?: string;
    systemPrompt?: string;
    parentAgentId?: string;
  };
  metrics: AgentMetrics;
  lastHeartbeat: Date;
  registeredAt: Date;
  updatedAt: Date;
}

export interface Task extends Document {
  _id?: ObjectId;
  id: string;
  type: AgentType;
  priority: TaskPriority;
  status: TaskStatus;
  payload: Record<string, any>;
  requiredCapabilities?: string[];
  assignedAgentId?: string;
  result?: any;
  error?: string;
  metadata: {
    source: string;
    userId?: string;
    projectId?: string;
    correlationId?: string;
    tags?: string[];
  };
  attempts: number;
  maxAttempts: number;
  timeout: number;
  createdAt: Date;
  assignedAt?: Date;
  startedAt?: Date;
  completedAt?: Date;
  updatedAt: Date;
}

export interface TaskAssignment {
  taskId: string;
  agentId: string;
  assignedAt: Date;
  expiresAt: Date;
}

export interface AgentMessage {
  id: string;
  from: string;
  to: string;
  type: 'request' | 'response' | 'event' | 'broadcast';
  topic: string;
  payload: any;
  correlationId?: string;
  timestamp: Date;
}

export interface AgentEvent {
  agentId: string;
  event: string;
  data?: any;
  timestamp: Date;
}

export interface TaskResult {
  taskId: string;
  agentId: string;
  status: 'success' | 'failure';
  result?: any;
  error?: {
    code: string;
    message: string;
    details?: any;
  };
  metrics?: {
    startTime: Date;
    endTime: Date;
    duration: number;
    resourcesUsed?: Record<string, any>;
  };
}

export interface AgentHealthCheck {
  agentId: string;
  status: 'healthy' | 'unhealthy' | 'degraded';
  checks: {
    connectivity: boolean;
    resources: boolean;
    capabilities: boolean;
  };
  details?: Record<string, any>;
  timestamp: Date;
}

export interface AgentRegistrationRequest {
  id?: string;
  name: string;
  type: AgentType;
  capabilities: AgentCapability[];
  endpoint?: string;
  metadata: {
    version: string;
    platform: string;
    region?: string;
    tags?: string[];
  };
}

export interface TaskRequest {
  type: AgentType;
  priority?: string | TaskPriority;
  payload: Record<string, any>;
  requiredCapabilities?: string[];
  metadata?: {
    source: string;
    userId?: string;
    projectId?: string;
    correlationId?: string;
    tags?: string[];
  };
  timeout?: number;
  maxAttempts?: number;
}

export interface AgentFilter {
  type?: AgentType;
  status?: AgentStatus;
  capabilities?: string[];
  tags?: string[];
  region?: string;
}

export interface TaskFilter {
  type?: AgentType;
  status?: TaskStatus;
  priority?: TaskPriority;
  agentId?: string;
  userId?: string;
  projectId?: string;
  dateRange?: {
    start: Date;
    end: Date;
  };
}