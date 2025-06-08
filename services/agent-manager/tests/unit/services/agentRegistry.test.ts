import { AgentRegistry } from '../../../src/services/agentRegistry';
import { MongoDBService } from '../../../src/services/mongodbService';
import { AgentType, AgentStatus, AgentRegistrationRequest } from '../../../src/models/agent';

// Mock dependencies
jest.mock('../../../src/services/mongodbService');
jest.mock('../../../src/utils/logger');

describe('AgentRegistry', () => {
  let agentRegistry: AgentRegistry;
  let mockMongoService: jest.Mocked<MongoDBService>;

  beforeEach(() => {
    mockMongoService = new MongoDBService() as jest.Mocked<MongoDBService>;
    mockMongoService.getCollection = jest.fn().mockReturnValue({
      find: jest.fn().mockReturnValue({
        toArray: jest.fn().mockResolvedValue([])
      }),
      insertOne: jest.fn().mockResolvedValue({ insertedId: '123' }),
      updateOne: jest.fn().mockResolvedValue({ modifiedCount: 1 }),
      bulkWrite: jest.fn().mockResolvedValue({ modifiedCount: 1 })
    });

    agentRegistry = new AgentRegistry(mockMongoService);
  });

  afterEach(async () => {
    await agentRegistry.cleanup();
  });

  describe('registerAgent', () => {
    it('should successfully register a new agent', async () => {
      const request: AgentRegistrationRequest = {
        name: 'Test Agent',
        type: AgentType.CODE_GEN,
        capabilities: [
          { name: 'javascript', version: '1.0' },
          { name: 'typescript', version: '1.0' }
        ],
        metadata: {
          version: '1.0.0',
          platform: 'linux',
          region: 'us-east-1',
          tags: ['test', 'unit']
        }
      };

      const agent = await agentRegistry.registerAgent(request, 'socket-123');

      expect(agent).toBeDefined();
      expect(agent.name).toBe(request.name);
      expect(agent.type).toBe(request.type);
      expect(agent.status).toBe(AgentStatus.AVAILABLE);
      expect(agent.socketId).toBe('socket-123');
      expect(agent.capabilities).toEqual(request.capabilities);
    });

    it('should emit agent:registered event', async () => {
      const request: AgentRegistrationRequest = {
        name: 'Test Agent',
        type: AgentType.TEST_GEN,
        capabilities: [{ name: 'jest', version: '29.0' }],
        metadata: {
          version: '1.0.0',
          platform: 'darwin'
        }
      };

      const eventSpy = jest.fn();
      agentRegistry.on('agent:registered', eventSpy);

      const agent = await agentRegistry.registerAgent(request);

      expect(eventSpy).toHaveBeenCalledWith(agent);
    });
  });

  describe('updateAgentStatus', () => {
    it('should update agent status', async () => {
      const request: AgentRegistrationRequest = {
        name: 'Test Agent',
        type: AgentType.DEPLOY,
        capabilities: [{ name: 'kubernetes', version: '1.28' }],
        metadata: {
          version: '1.0.0',
          platform: 'linux'
        }
      };

      const agent = await agentRegistry.registerAgent(request);
      await agentRegistry.updateAgentStatus(agent.id, AgentStatus.BUSY);

      const updatedAgent = agentRegistry.getAgent(agent.id);
      expect(updatedAgent?.status).toBe(AgentStatus.BUSY);
    });

    it('should throw error for non-existent agent', async () => {
      await expect(
        agentRegistry.updateAgentStatus('non-existent-id', AgentStatus.BUSY)
      ).rejects.toThrow('Agent non-existent-id not found');
    });
  });

  describe('findBestAgent', () => {
    it('should find agent with required capabilities', async () => {
      // Register multiple agents
      const agent1 = await agentRegistry.registerAgent({
        name: 'Agent 1',
        type: AgentType.CODE_GEN,
        capabilities: [{ name: 'javascript', version: '1.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      const agent2 = await agentRegistry.registerAgent({
        name: 'Agent 2',
        type: AgentType.CODE_GEN,
        capabilities: [
          { name: 'javascript', version: '1.0' },
          { name: 'typescript', version: '1.0' }
        ],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      // Find agent with TypeScript capability
      const bestAgent = agentRegistry.findBestAgent(
        AgentType.CODE_GEN,
        ['typescript']
      );

      expect(bestAgent).toBeDefined();
      expect(bestAgent?.id).toBe(agent2.id);
    });

    it('should return null if no agent matches requirements', () => {
      const bestAgent = agentRegistry.findBestAgent(
        AgentType.CODE_GEN,
        ['non-existent-capability']
      );

      expect(bestAgent).toBeNull();
    });

    it('should prefer agents with better metrics', async () => {
      // Register two agents
      const agent1 = await agentRegistry.registerAgent({
        name: 'Slow Agent',
        type: AgentType.TEST_GEN,
        capabilities: [{ name: 'jest', version: '1.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      const agent2 = await agentRegistry.registerAgent({
        name: 'Fast Agent',
        type: AgentType.TEST_GEN,
        capabilities: [{ name: 'jest', version: '1.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      // Update metrics
      await agentRegistry.updateAgentMetrics(agent1.id, {
        averageResponseTime: 5000,
        tasksCompleted: 10,
        tasksFailed: 2
      });

      await agentRegistry.updateAgentMetrics(agent2.id, {
        averageResponseTime: 1000,
        tasksCompleted: 20,
        tasksFailed: 1
      });

      const bestAgent = agentRegistry.findBestAgent(AgentType.TEST_GEN);
      expect(bestAgent?.id).toBe(agent2.id);
    });
  });

  describe('getAvailableAgents', () => {
    it('should return only available agents of specified type', async () => {
      const agent1 = await agentRegistry.registerAgent({
        name: 'Available Agent',
        type: AgentType.MONITOR,
        capabilities: [{ name: 'prometheus', version: '2.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      const agent2 = await agentRegistry.registerAgent({
        name: 'Busy Agent',
        type: AgentType.MONITOR,
        capabilities: [{ name: 'grafana', version: '9.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      const agent3 = await agentRegistry.registerAgent({
        name: 'Different Type',
        type: AgentType.SECURITY,
        capabilities: [{ name: 'sonarqube', version: '9.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      // Update agent2 to busy
      await agentRegistry.updateAgentStatus(agent2.id, AgentStatus.BUSY);

      const availableAgents = agentRegistry.getAvailableAgents(AgentType.MONITOR);

      expect(availableAgents).toHaveLength(1);
      expect(availableAgents[0].id).toBe(agent1.id);
    });
  });

  describe('heartbeat monitoring', () => {
    it('should update agent heartbeat', async () => {
      const agent = await agentRegistry.registerAgent({
        name: 'Test Agent',
        type: AgentType.OPTIMIZATION,
        capabilities: [{ name: 'performance', version: '1.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      const initialHeartbeat = agent.lastHeartbeat;
      
      // Wait a bit
      await new Promise(resolve => setTimeout(resolve, 100));
      
      await agentRegistry.updateAgentHeartbeat(agent.id);
      const updatedAgent = agentRegistry.getAgent(agent.id);

      expect(updatedAgent?.lastHeartbeat.getTime()).toBeGreaterThan(
        initialHeartbeat.getTime()
      );
    });

    it('should mark offline agent as available on heartbeat', async () => {
      const agent = await agentRegistry.registerAgent({
        name: 'Test Agent',
        type: AgentType.REVIEW,
        capabilities: [{ name: 'eslint', version: '8.0' }],
        metadata: { version: '1.0.0', platform: 'linux' }
      });

      // Mark as offline
      await agentRegistry.updateAgentStatus(agent.id, AgentStatus.OFFLINE);
      
      // Send heartbeat
      await agentRegistry.updateAgentHeartbeat(agent.id);
      
      const updatedAgent = agentRegistry.getAgent(agent.id);
      expect(updatedAgent?.status).toBe(AgentStatus.AVAILABLE);
    });
  });

  describe('findAgents', () => {
    beforeEach(async () => {
      // Register test agents
      await agentRegistry.registerAgent({
        name: 'US Agent 1',
        type: AgentType.CODE_GEN,
        capabilities: [
          { name: 'javascript', version: '1.0' },
          { name: 'python', version: '3.0' }
        ],
        metadata: {
          version: '1.0.0',
          platform: 'linux',
          region: 'us-east-1',
          tags: ['production', 'high-performance']
        }
      });

      await agentRegistry.registerAgent({
        name: 'EU Agent 1',
        type: AgentType.CODE_GEN,
        capabilities: [{ name: 'javascript', version: '1.0' }],
        metadata: {
          version: '1.0.0',
          platform: 'linux',
          region: 'eu-west-1',
          tags: ['staging']
        }
      });

      await agentRegistry.registerAgent({
        name: 'US Agent 2',
        type: AgentType.TEST_GEN,
        capabilities: [{ name: 'jest', version: '29.0' }],
        metadata: {
          version: '1.0.0',
          platform: 'darwin',
          region: 'us-east-1',
          tags: ['production']
        }
      });
    });

    it('should filter by type', () => {
      const agents = agentRegistry.findAgents({ type: AgentType.CODE_GEN });
      expect(agents).toHaveLength(2);
      expect(agents.every(a => a.type === AgentType.CODE_GEN)).toBe(true);
    });

    it('should filter by region', () => {
      const agents = agentRegistry.findAgents({ region: 'us-east-1' });
      expect(agents).toHaveLength(2);
      expect(agents.every(a => a.metadata.region === 'us-east-1')).toBe(true);
    });

    it('should filter by capabilities', () => {
      const agents = agentRegistry.findAgents({ capabilities: ['python'] });
      expect(agents).toHaveLength(1);
      expect(agents[0].name).toBe('US Agent 1');
    });

    it('should filter by tags', () => {
      const agents = agentRegistry.findAgents({ tags: ['production'] });
      expect(agents).toHaveLength(2);
      expect(agents.every(a => a.metadata.tags?.includes('production'))).toBe(true);
    });

    it('should apply multiple filters', () => {
      const agents = agentRegistry.findAgents({
        type: AgentType.CODE_GEN,
        region: 'us-east-1',
        tags: ['production']
      });
      expect(agents).toHaveLength(1);
      expect(agents[0].name).toBe('US Agent 1');
    });
  });
});