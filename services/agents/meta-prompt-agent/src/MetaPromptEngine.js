const { SystemMessage, HumanMessage } = require('langchain/schema');
const winston = require('winston');
const LLMProviderFactory = require('./LLMProviderFactory');

class MetaPromptEngine {
  constructor(config) {
    this.config = config;
    this.logger = winston.createLogger({
      level: 'info',
      format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
      ),
      transports: [new winston.transports.Console()]
    });

    // Initialize LLM provider factory
    this.llmFactory = new LLMProviderFactory();
    
    // Get the default LLM or use specified provider
    const provider = config.llmProvider || this.llmFactory.defaultProvider;
    const model = config.llmModel;
    
    this.logger.info('Initializing MetaPromptEngine with LLM provider', { provider, model });
    
    // Initialize LLM for meta-prompt processing
    this.llm = this.llmFactory.createLLM({
      provider,
      model,
      config: {
        temperature: config.temperature || 0.7,
        maxTokens: config.maxTokens || 2000
      }
    });

    // Meta-prompt templates for different purposes
    this.metaTemplates = {
      agentDesign: `You are an expert AI agent architect. Given a task description and requirements, design an optimal AI agent configuration.

Task: {task}
Requirements: {requirements}
Context: {context}

Design an agent with:
1. Core purpose and capabilities
2. Detailed system prompt that will guide the agent's behavior
3. Input/output specifications
4. Error handling strategies
5. Performance optimization hints

Return as JSON:
{
  "name": "descriptive agent name",
  "type": "agent category",
  "purpose": "clear purpose statement",
  "systemPrompt": "detailed system prompt for the agent",
  "capabilities": ["capability1", "capability2"],
  "inputSchema": {},
  "outputSchema": {},
  "behaviorModifiers": {
    "temperature": 0.7,
    "maxTokens": 1000,
    "topP": 1,
    "frequencyPenalty": 0,
    "presencePenalty": 0
  },
  "errorHandling": {
    "retryStrategy": "exponential|linear|none",
    "maxRetries": 3,
    "fallbackBehavior": "description"
  }
}`,

      promptOptimization: `You are a prompt engineering expert. Analyze and optimize the given prompt for better performance.

Current Prompt: {currentPrompt}
Performance Metrics: {metrics}
User Feedback: {feedback}
Task Examples: {examples}

Optimize the prompt considering:
1. Clarity and specificity
2. Output format consistency
3. Edge case handling
4. Token efficiency
5. Response quality

Provide:
1. Optimized prompt
2. Explanation of changes
3. Expected improvements`,

      capabilityExtraction: `Analyze the given agent prompt and extract its capabilities in a structured format.

Agent Prompt: {prompt}
Agent Type: {agentType}

Extract:
1. Primary capabilities (what the agent can do)
2. Input requirements
3. Output formats
4. Constraints and limitations
5. Performance characteristics

Return as JSON with capability descriptions.`,

      workflowDecomposition: `You are a workflow architect. Given a complex task, decompose it into agent-executable steps.

Task: {task}
Available Agent Types: {agentTypes}
Constraints: {constraints}

Create a workflow with:
1. Step-by-step breakdown
2. Agent assignments for each step
3. Data flow between steps
4. Parallel execution opportunities
5. Error handling checkpoints

Return as structured workflow definition.`
    };

    // Prompt performance tracking
    this.promptMetrics = new Map();
    
    // Agent instance tracking
    this.activeAgents = new Map();
  }

  /**
   * Design a new agent based on task requirements
   */
  async designAgent(task, requirements = {}, context = {}) {
    this.logger.info('Designing new agent', { task, requirements });

    try {
      const messages = [
        new SystemMessage(this.metaTemplates.agentDesign),
        new HumanMessage(JSON.stringify({
          task,
          requirements,
          context
        }))
      ];

      const response = await this.llm.invoke(messages);
      const agentDesign = JSON.parse(response.content);

      // Validate and enhance the design
      const enhancedDesign = await this.enhanceAgentDesign(agentDesign);
      
      this.logger.info('Agent design completed', { 
        agentName: enhancedDesign.name,
        capabilities: enhancedDesign.capabilities 
      });

      return enhancedDesign;
    } catch (error) {
      this.logger.error('Failed to design agent', { error: error.message });
      throw error;
    }
  }

  /**
   * Optimize an existing prompt based on performance data
   */
  async optimizePrompt(currentPrompt, performanceData = {}) {
    this.logger.info('Optimizing prompt based on performance data');

    const messages = [
      new SystemMessage(this.metaTemplates.promptOptimization),
      new HumanMessage(JSON.stringify({
        currentPrompt,
        metrics: performanceData.metrics || {},
        feedback: performanceData.feedback || [],
        examples: performanceData.examples || []
      }))
    ];

    const response = await this.llm.invoke(messages);
    return this.parseOptimizationResponse(response.content);
  }

  /**
   * Create a dynamic agent instance
   */
  async createDynamicAgent(config) {
    const { id, design, taskQueue } = config;
    
    const agent = new DynamicAgent({
      id,
      design,
      taskQueue,
      llm: this.createAgentLLM(design.behaviorModifiers),
      logger: this.logger
    });

    this.activeAgents.set(id, agent);
    return agent;
  }

  /**
   * Extract capabilities from an agent prompt
   */
  async extractCapabilities(prompt, agentType = 'general') {
    const messages = [
      new SystemMessage(this.metaTemplates.capabilityExtraction),
      new HumanMessage(JSON.stringify({ prompt, agentType }))
    ];

    const response = await this.llm.invoke(messages);
    return JSON.parse(response.content);
  }

  /**
   * Decompose a complex task into agent-executable workflow
   */
  async decomposeTask(task, availableAgentTypes, constraints = {}) {
    const messages = [
      new SystemMessage(this.metaTemplates.workflowDecomposition),
      new HumanMessage(JSON.stringify({
        task,
        agentTypes: availableAgentTypes,
        constraints
      }))
    ];

    const response = await this.llm.invoke(messages);
    return JSON.parse(response.content);
  }

  /**
   * Enhance agent design with additional metadata
   */
  async enhanceAgentDesign(design) {
    // Add version tracking
    design.version = '1.0.0';
    design.createdAt = new Date().toISOString();
    
    // Add performance baselines
    design.performanceBaselines = {
      avgResponseTime: null,
      successRate: null,
      tokenUsage: null
    };

    // Add evolution tracking
    design.evolution = {
      generation: 1,
      parentId: null,
      improvements: []
    };

    // Validate required fields
    const requiredFields = ['name', 'type', 'systemPrompt', 'capabilities'];
    for (const field of requiredFields) {
      if (!design[field]) {
        throw new Error(`Missing required field: ${field}`);
      }
    }

    return design;
  }

  /**
   * Create an LLM instance with specific behavior modifiers
   */
  createAgentLLM(behaviorModifiers = {}) {
    // Allow agents to specify their preferred provider/model
    const provider = behaviorModifiers.provider || this.config.llmProvider || this.llmFactory.defaultProvider;
    const model = behaviorModifiers.model || this.config.llmModel;
    
    return this.llmFactory.createLLM({
      provider,
      model,
      config: {
        temperature: behaviorModifiers.temperature || 0.7,
        maxTokens: behaviorModifiers.maxTokens || 1000,
        topP: behaviorModifiers.topP || 1,
        frequencyPenalty: behaviorModifiers.frequencyPenalty || 0,
        presencePenalty: behaviorModifiers.presencePenalty || 0
      }
    });
  }

  /**
   * Parse optimization response
   */
  parseOptimizationResponse(content) {
    try {
      // Try to parse as JSON first
      return JSON.parse(content);
    } catch {
      // Fall back to text parsing
      const sections = content.split('\n\n');
      return {
        optimizedPrompt: sections[0] || content,
        explanation: sections[1] || 'No explanation provided',
        improvements: sections[2] || 'No specific improvements listed'
      };
    }
  }

  /**
   * Track prompt performance
   */
  trackPerformance(agentId, promptVersion, metrics) {
    const key = `${agentId}-${promptVersion}`;
    const existing = this.promptMetrics.get(key) || {
      totalRuns: 0,
      totalTokens: 0,
      totalTime: 0,
      errors: 0,
      feedback: []
    };

    existing.totalRuns++;
    existing.totalTokens += metrics.tokens || 0;
    existing.totalTime += metrics.executionTime || 0;
    if (metrics.error) existing.errors++;
    if (metrics.feedback) existing.feedback.push(metrics.feedback);

    this.promptMetrics.set(key, existing);
  }

  /**
   * Get performance summary for an agent
   */
  getPerformanceSummary(agentId) {
    const summary = {
      versions: [],
      overall: {
        totalRuns: 0,
        avgTokens: 0,
        avgTime: 0,
        errorRate: 0
      }
    };

    for (const [key, metrics] of this.promptMetrics) {
      if (key.startsWith(agentId)) {
        const version = key.split('-')[1];
        summary.versions.push({
          version,
          metrics: {
            runs: metrics.totalRuns,
            avgTokens: metrics.totalTokens / metrics.totalRuns,
            avgTime: metrics.totalTime / metrics.totalRuns,
            errorRate: metrics.errors / metrics.totalRuns
          }
        });

        summary.overall.totalRuns += metrics.totalRuns;
      }
    }

    return summary;
  }
}

/**
 * Dynamic Agent that operates based on prompts
 */
class DynamicAgent {
  constructor(config) {
    this.id = config.id;
    this.design = config.design;
    this.taskQueue = config.taskQueue;
    this.llm = config.llm;
    this.logger = config.logger;
    this.status = 'idle';
    this.currentTask = null;
  }

  /**
   * Execute a task using the agent's prompt
   */
  async executeTask(task) {
    this.status = 'busy';
    this.currentTask = task;
    const startTime = Date.now();

    try {
      // Construct messages with system prompt and task
      const messages = [
        new SystemMessage(this.design.systemPrompt),
        new HumanMessage(JSON.stringify(task.input))
      ];

      // Add context if provided
      if (task.context) {
        messages.push(new SystemMessage(`Context: ${JSON.stringify(task.context)}`));
      }

      // Execute with LLM
      const response = await this.llm.invoke(messages);
      
      // Parse and validate response
      const result = this.parseResponse(response.content);
      
      // Track performance
      const metrics = {
        executionTime: Date.now() - startTime,
        tokens: response.usage?.total_tokens || 0,
        success: true
      };

      return {
        success: true,
        result,
        metrics
      };
    } catch (error) {
      this.logger.error('Task execution failed', { 
        agentId: this.id, 
        taskId: task.id, 
        error: error.message 
      });

      return {
        success: false,
        error: error.message,
        metrics: {
          executionTime: Date.now() - startTime,
          error: true
        }
      };
    } finally {
      this.status = 'idle';
      this.currentTask = null;
    }
  }

  /**
   * Parse agent response based on expected output schema
   */
  parseResponse(content) {
    if (this.design.outputSchema) {
      try {
        return JSON.parse(content);
      } catch {
        // If JSON parsing fails, return as text
        return { output: content };
      }
    }
    return content;
  }

  /**
   * Get agent status and metadata
   */
  getStatus() {
    return {
      id: this.id,
      name: this.design.name,
      type: this.design.type,
      status: this.status,
      capabilities: this.design.capabilities,
      currentTask: this.currentTask?.id || null,
      version: this.design.version
    };
  }
}

module.exports = { MetaPromptEngine, DynamicAgent };