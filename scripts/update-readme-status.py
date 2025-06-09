#!/usr/bin/env python3
"""
Update README.md with current system status
"""

import subprocess
import json
import requests
import os
from datetime import datetime

def check_service_health(service_name, url):
    """Check if a service is healthy"""
    try:
        response = requests.get(url, timeout=5)
        return response.status_code == 200
    except:
        return False

def get_docker_services():
    """Get status of docker services"""
    try:
        result = subprocess.run(
            ['docker-compose', '-f', 'docker-compose.minimal.yml', 'ps', '--format', 'json'],
            capture_output=True,
            text=True
        )
        if result.returncode == 0:
            services = json.loads(result.stdout)
            return {s['Service']: s['State'] == 'running' for s in services}
    except:
        return {}

def count_todos():
    """Count TODOs in codebase"""
    try:
        result = subprocess.run(
            ['grep', '-r', 'TODO', '--include=*.go', '--include=*.ts', '--include=*.py', '.'],
            capture_output=True,
            text=True
        )
        return len(result.stdout.strip().split('\n')) if result.stdout else 0
    except:
        return 0

def get_test_coverage():
    """Get test coverage percentage"""
    # This would run actual test coverage tools
    # For now, return a placeholder
    return "Pending Implementation"

def update_readme():
    """Update README.md with current status"""
    
    # Check if running in CI or local
    is_ci = os.getenv('CI', 'false') == 'true'
    
    if is_ci:
        # In CI, use placeholder data
        services_status = {
            'orchestrator': 'unknown',
            'agent-manager': 'unknown',
            'intent-processor': 'unknown'
        }
        todos = 'N/A'
        last_updated = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S UTC")
    else:
        # Local development
        services = get_docker_services()
        services_status = {
            'orchestrator': '✅' if services.get('orchestrator', False) else '❌',
            'agent-manager': '✅' if services.get('agent-manager', False) else '❌',
            'intent-processor': '✅' if services.get('intent-processor', False) else '❌'
        }
        todos = count_todos()
        last_updated = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    status_section = f"""
## 📊 System Status

> Last Updated: {last_updated}

### 🏃 Service Health
| Service | Status | Description |
|---------|--------|-------------|
| Orchestrator | {services_status['orchestrator']} | Workflow orchestration service |
| Agent Manager | {services_status['agent-manager']} | Agent lifecycle management |
| Intent Processor | {services_status['intent-processor']} | Natural language processing |

### 📈 Code Quality
- **TODOs in Codebase**: {todos}
- **Test Coverage**: {get_test_coverage()}
- **Security Vulnerabilities**: Check [Security Tab](../../security)

### 🚀 Quick Start
```bash
# Start all services
make up

# Check health
make health

# Run demo
make demo
```
"""
    
    # Read current README
    with open('README.md', 'r') as f:
        readme = f.read()
    
    # Find status section markers
    start_marker = "<!-- STATUS_START -->"
    end_marker = "<!-- STATUS_END -->"
    
    if start_marker in readme and end_marker in readme:
        # Replace existing status section
        start_idx = readme.find(start_marker) + len(start_marker)
        end_idx = readme.find(end_marker)
        new_readme = readme[:start_idx] + status_section + readme[end_idx:]
    else:
        # Add status section after first heading
        lines = readme.split('\n')
        for i, line in enumerate(lines):
            if line.startswith('# '):
                lines.insert(i + 1, f"\n{start_marker}{status_section}{end_marker}\n")
                break
        new_readme = '\n'.join(lines)
    
    # Write updated README
    with open('README.md', 'w') as f:
        f.write(new_readme)
    
    print(f"✅ README.md updated with current status")

if __name__ == "__main__":
    update_readme()