#!/usr/bin/env ts-node

import { ServiceInitializer } from './services/agent-manager/src/services/serviceInitializer';
import { AgentStatus } from './services/agent-manager/src/models/agent';

async function testRobustRegistry() {
  console.log('=== Testing Robust Agent Registry ===\n');

  const initializer = new ServiceInitializer({
    mongoUri: process.env.MONGODB_URI || 'mongodb://admin:mongo123@localhost:27017/agent_manager?authSource=admin',
    dbName: 'agent_manager',
    redisUrl: process.env.REDIS_URL || 'redis://:redis123@localhost:6379',
    useV2Registry: true
  });

  try {
    // Initialize services
    console.log('1. Initializing services...');
    const { registry, mongoService } = await initializer.initialize();
    console.log('✓ Services initialized\n');

    // Test agent registration
    console.log('2. Testing agent registration...');
    const testAgent = await registry.registerAgent({
      id: 'test-agent-001',
      name: 'Test Agent',
      type: 'test' as any,
      capabilities: [{
        name: 'test-capability',
        description: 'Test capability',
        version: '1.0.0'
      }],
      metadata: {
        version: '1.0.0',
        platform: 'nodejs',
        region: 'test'
      }
    });
    console.log('✓ Agent registered:', testAgent.id);

    // Test agent retrieval
    console.log('\n3. Testing agent retrieval...');
    const retrievedAgent = await registry.getAgent('test-agent-001');
    console.log('✓ Agent retrieved:', retrievedAgent?.id);

    // Test status update
    console.log('\n4. Testing status update...');
    await registry.updateAgentStatus('test-agent-001', AgentStatus.BUSY);
    console.log('✓ Status updated to BUSY');

    // Test heartbeat
    console.log('\n5. Testing heartbeat update...');
    await registry.updateAgentHeartbeat('test-agent-001');
    console.log('✓ Heartbeat updated');

    // Test getting all agents
    console.log('\n6. Testing get all agents...');
    const allAgents = registry.getAllAgents();
    console.log(`✓ Found ${allAgents.length} agents`);

    // Simulate disconnection and reconnection
    console.log('\n7. Testing disconnection/reconnection...');
    
    // Mark offline
    await registry.updateAgentStatus('test-agent-001', AgentStatus.OFFLINE);
    console.log('✓ Agent marked offline');

    // Simulate reconnection by updating status
    await registry.updateAgentStatus('test-agent-001', AgentStatus.AVAILABLE);
    console.log('✓ Agent reconnected and marked available');

    // Test concurrent operations
    console.log('\n8. Testing concurrent operations...');
    const promises = [];
    for (let i = 0; i < 5; i++) {
      promises.push(registry.updateAgentHeartbeat('test-agent-001'));
    }
    await Promise.all(promises);
    console.log('✓ Handled 5 concurrent heartbeat updates');

    // Clean up
    console.log('\n9. Cleaning up...');
    await registry.unregisterAgent('test-agent-001');
    console.log('✓ Test agent unregistered');

    await initializer.cleanup();
    console.log('✓ Services cleaned up');

    console.log('\n=== All tests passed! ===');

  } catch (error) {
    console.error('\n✗ Test failed:', error);
    await initializer.cleanup();
    process.exit(1);
  }
}

// Run the test
testRobustRegistry().catch(console.error);