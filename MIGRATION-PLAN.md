# Agent Registry Migration Plan

## Overview

This document outlines the migration from the current in-memory agent registry to a robust, distributed agent registry that can scale with the platform.

## Current Issues

1. **State Synchronization**: Agents exist in MongoDB but not in the in-memory registry after restarts
2. **Race Conditions**: Multiple services trying to register/update the same agent
3. **No Distributed Support**: Registry state is local to each agent-manager instance
4. **Socket.IO Persistence**: Connections drop and agents need to re-register

## New Architecture

### Components

1. **AgentRepository**: 
   - Handles all database operations
   - Implements caching with Redis
   - Publishes events for state changes

2. **AgentRegistryV2**:
   - Maintains local cache for performance
   - Subscribes to Redis events for real-time updates
   - Implements automatic synchronization
   - Handles heartbeat monitoring

3. **Improved AgentCommunicator**:
   - Requests agent info if not found
   - Handles registration asynchronously
   - More resilient to connection issues

### Key Features

- **Distributed State**: Uses Redis for cache and event propagation
- **Automatic Recovery**: Agents loaded from DB if not in memory
- **Event Sourcing**: All state changes published as events
- **Resilient Connections**: Better handling of disconnections
- **Performance**: Local cache with background sync

## Migration Steps

### Phase 1: Preparation (Current)
✅ Create new components (AgentRepository, AgentRegistryV2)
✅ Add Redis support
✅ Update AgentCommunicator for better registration
✅ Add backward compatibility

### Phase 2: Testing
1. Deploy V2 registry in test environment
2. Run integration tests
3. Test failover scenarios
4. Performance benchmarking

### Phase 3: Gradual Rollout
1. Enable V2 registry with feature flag
2. Monitor metrics and logs
3. Rollback plan ready
4. Gradual increase in traffic

### Phase 4: Full Migration
1. Enable V2 for all environments
2. Monitor for 1 week
3. Remove legacy code
4. Update documentation

## Configuration

```yaml
# Enable V2 Registry
USE_V2_REGISTRY: true

# Redis Configuration
REDIS_URL: redis://redis:6379
REDIS_CACHE_TTL: 300

# Sync Settings
REGISTRY_SYNC_INTERVAL: 5000
HEARTBEAT_TIMEOUT: 30000
HEARTBEAT_CHECK_INTERVAL: 10000
```

## Benefits

1. **Scalability**: Can handle thousands of agents
2. **Reliability**: Automatic recovery from failures
3. **Performance**: Redis caching reduces DB load
4. **Real-time**: Event-driven updates across instances
5. **Monitoring**: Better visibility into agent states

## Rollback Plan

If issues arise during migration:

1. Set `USE_V2_REGISTRY=false`
2. Restart agent-manager services
3. Verify agents reconnect properly
4. Investigate issues in staging

## Monitoring

Key metrics to watch:

- Agent registration time
- Heartbeat processing latency
- Redis memory usage
- MongoDB query performance
- Socket.IO connection stability

## Timeline

- Week 1: Testing in development
- Week 2: Staging deployment
- Week 3: Production canary (10%)
- Week 4: Full production rollout

## Success Criteria

- Zero agent registration failures
- < 100ms registration latency
- 99.9% heartbeat success rate
- Successful failover testing
- No increase in error rates