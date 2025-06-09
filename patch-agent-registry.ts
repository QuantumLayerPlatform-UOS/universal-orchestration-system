// Add this method to AgentRegistry class
public async initialize(): Promise<void> {
  await this.loadAgentsFromDatabase();
}

// In index.ts, after MongoDB connection:
// await this.mongoService.connect();
// logger.info('Connected to MongoDB');
// 
// // Initialize agent registry
// await this.agentRegistry.initialize();
// logger.info('Agent registry initialized');