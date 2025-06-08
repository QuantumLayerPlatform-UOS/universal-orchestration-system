import dotenv from 'dotenv';

// Load test environment variables
dotenv.config({ path: '.env.test' });

// Set test environment
process.env.NODE_ENV = 'test';
process.env.LOG_LEVEL = 'error'; // Reduce noise in tests

// Mock Redis URL if not set
if (!process.env.REDIS_URL) {
  process.env.REDIS_URL = 'redis://localhost:6379';
}

// Mock MongoDB URI if not set
if (!process.env.MONGODB_URI) {
  process.env.MONGODB_URI = 'mongodb://localhost:27017/agent-manager-test';
}

// Global test timeout
jest.setTimeout(30000);

// Clean up after all tests
afterAll(async () => {
  // Close any open handles
  await new Promise(resolve => setTimeout(resolve, 500));
});