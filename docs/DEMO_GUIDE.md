# 🎭 QLP-UOS Demo Guide

> Always be demo-ready! This guide ensures you can demonstrate the platform at any time.

## 🚀 Quick Demo (< 5 minutes)

```bash
# Start everything
make quick

# This will:
# 1. Start all services
# 2. Wait for health checks
# 3. Launch interactive demo
```

## 📋 Demo Scenarios

### 1. REST API Generation (2 minutes)
**Pitch**: "From description to working API in seconds"

```bash
make demo-api
```

**What it shows**:
- Natural language understanding
- Task breakdown
- Code generation capabilities

**Expected Output**:
```json
{
  "intent_type": "CREATE_FEATURE",
  "tasks": [
    {
      "name": "Design API endpoints",
      "type": "DESIGN",
      "estimated_effort": 120
    },
    {
      "name": "Implement API routes",
      "type": "IMPLEMENTATION",
      "estimated_effort": 240
    }
  ]
}
```

### 2. Bug Fix Workflow (3 minutes)
**Pitch**: "AI-powered debugging and fixing"

```bash
make demo-bugfix
```

**What it shows**:
- Intent classification
- Agent orchestration
- Automated fix generation

### 3. Test Suite Generation (2 minutes)
**Pitch**: "Comprehensive testing without the manual work"

```bash
make demo-tests
```

**What it shows**:
- Test scenario generation
- Coverage analysis
- Multiple testing frameworks

### 4. Full End-to-End (10 minutes)
**Pitch**: "From idea to deployed application"

```bash
make demo-full
```

**What it shows**:
- Complete workflow
- Agent collaboration
- Deployment pipeline

## 🎯 Demo Best Practices

### Before the Demo

1. **Health Check**
   ```bash
   make health
   ```
   All services should show ✅

2. **Clear Logs**
   ```bash
   docker-compose -f docker-compose.minimal.yml logs --tail=0 -f
   ```
   Keep this running in a separate terminal

3. **Prepare Data**
   ```bash
   make demo-prepare
   ```
   Ensures sample data is loaded

### During the Demo

1. **Start with the Problem**
   - "Traditional development takes weeks/months"
   - "We do it in minutes"

2. **Show Don't Tell**
   - Run the actual commands
   - Show real-time logs
   - Display generated artifacts

3. **Handle Failures Gracefully**
   - If something fails: "This is why we have resilience patterns"
   - Run `make quick-fix` if needed

### After the Demo

1. **Show Artifacts**
   - Generated code
   - Test results
   - Deployment status

2. **Metrics**
   ```bash
   make demo-metrics
   ```
   Shows time saved, code quality, etc.

## 🔧 Troubleshooting

### Services Not Starting
```bash
make down
make clean-docker
make up
```

### Health Check Failures
```bash
# Check specific service
docker logs qlp-uos-orchestrator-1 --tail=50

# Restart specific service
docker-compose -f docker-compose.minimal.yml restart orchestrator
```

### Demo Data Issues
```bash
# Reset demo environment
make demo-reset
```

## 📊 Demo Metrics

Track these KPIs during demos:

- **Time to First Code**: < 30 seconds
- **Intent Recognition Accuracy**: > 95%
- **Service Uptime**: 100% during demo
- **Response Time**: < 2 seconds

## 🎪 Advanced Demos

### Live Coding Session
```bash
# Watch the AI code in real-time
make demo-live-coding
```

### Multi-Agent Collaboration
```bash
# Show agents working together
make demo-ensemble
```

### Cost Optimization
```bash
# Show cost tracking
make demo-cost
```

## 📝 Demo Scripts

### 30-Second Elevator Pitch
"QuantumLayer transforms ideas into code. Watch this: [run quick demo]. That took 30 seconds. Traditional development? 3 weeks."

### 2-Minute Investor Demo
1. Problem: "Software development is too slow and expensive"
2. Solution: "AI agents that understand intent and generate code"
3. Demo: Run REST API generation
4. Traction: "Already saving 90% development time"
5. Ask: "Let's discuss how we scale this"

### 10-Minute Technical Demo
1. Architecture overview (1 min)
2. Live API generation (2 min)
3. Show generated code (1 min)
4. Run tests (1 min)
5. Deploy to cloud (2 min)
6. Show monitoring (1 min)
7. Q&A (2 min)

## 🚨 Emergency Commands

If something goes wrong during a demo:

```bash
# Quick recovery
make quick-fix

# Full reset (30 seconds)
make demo-emergency-reset

# Fallback to recorded demo
make demo-video
```

---

Remember: **Confidence sells**. If something breaks, explain it's a feature (resilience testing)! 🎭