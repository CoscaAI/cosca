# Example: Custom Workflow

> **Status**: active | **Owner**: Workflow Chief | **Last Updated**: 2026-07-23

This example demonstrates creating and running a custom workflow with Cosca. Workflows automate multi-step processes with defined states, transitions, hooks, and validation.

---

## Workflow Overview

We'll build a **Code Review Workflow** that:

1. Triggers when a pull request is created
2. Runs static analysis and linting
3. Performs knowledge-based code review
4. Generates a review report
5. Stores the review in memory for future reference

### Workflow States

```
REQUESTED → ANALYSIS → REVIEWING → REPORTING → COMPLETED
    │          │          │            │
    ▼          ▼          ▼            ▼
  ERROR ←── ERROR ←─── ERROR ←───── ERROR
```

---

## Step 1: Create Workflow Directory

```bash
mkdir -p .cosca/workflows
```

---

## Step 2: Define the Workflow

Create `.cosca/workflows/code-review.yaml`:

```yaml
# .cosca/workflows/code-review.yaml

# Workflow metadata
name: "code-review"
version: "1.0.0"
description: "Automated code review with static analysis and knowledge base lookup"
author: "Cosca"

# Workflow configuration
config:
  max_retries: 3
  timeout: "5m"
  concurrency: 1
  notify_on:
    - completed
    - error

# Input parameters
inputs:
  - name: "branch"
    type: "string"
    required: true
    description: "Branch name to review"
  - name: "base_branch"
    type: "string"
    required: false
    default: "main"
    description: "Base branch for comparison"
  - name: "severity"
    type: "string"
    required: false
    default: "all"
    enum: ["all", "critical", "high", "medium", "low"]
    description: "Minimum severity to report"

# Output artifacts
outputs:
  - name: "review_report"
    type: "file"
    path: ".cosca/reports/review-{{ .timestamp }}.md"
  - name: "review_summary"
    type: "memory"
    key: "code-review-{{ .branch }}"

# Preconditions
preconditions:
  - "Git repository is initialized"
  - "Branch {{ .branch }} exists"
  - "Base branch {{ .base_branch }} exists"
  - "Cosca knowledge base is indexed"

# Postconditions
postconditions:
  - "Review report generated at .cosca/reports/"
  - "Review summary stored in Cosca memory"
  - "Plugin hooks executed for review results"

# Steps
steps:
  - id: "analyze"
    name: "Static Analysis"
    description: "Run linters and static analysis tools"
    timeout: "60s"
    retries: 2
    command: |
      #!/bin/bash
      echo "Running static analysis on branch {{ .branch }}..."
      
      # Run language-specific linters
      if [ -f "go.mod" ]; then
        go vet ./... 2>&1 | tee .cosca/reports/vet-output.txt
      fi
      if [ -f "package.json" ]; then
        npx eslint src/ --format json 2>&1 | tee .cosca/reports/eslint-output.json
      fi
      
      # Parse results
      python3 -c "
      import json, os
      
      issues = []
      
      # Parse go vet output
      vet_file = '.cosca/reports/vet-output.txt'
      if os.path.exists(vet_file):
          with open(vet_file) as f:
              for line in f:
                  if line.strip() and ':' in line:
                      parts = line.split(':')
                      issues.append({
                          'tool': 'go vet',
                          'file': parts[0],
                          'line': int(parts[1]) if parts[1].isdigit() else 0,
                          'message': ':'.join(parts[2:]).strip()
                      })
      
      # Save parsed issues
      with open('.cosca/reports/analysis-results.json', 'w') as f:
          json.dump({'issues': issues, 'total': len(issues)}, f)
      
      print(f'Found {len(issues)} issue(s)')
      "
    outputs:
      issues_file: ".cosca/reports/analysis-results.json"

  - id: "review"
    name: "Knowledge-Based Review"
    description: "Search knowledge base for relevant patterns and best practices"
    timeout: "120s"
    depends_on: ["analyze"]
    command: |
      #!/bin/bash
      echo "Performing knowledge-based code review..."
      
      # Get list of changed files
      git diff --name-only "origin/{{ .base_branch }}...{{ .branch }}" > .cosca/reports/changed-files.txt
      
      # Search knowledge base for each changed file
      while IFS= read -r file; do
        if [ -n "$file" ]; then
          echo "Reviewing: $file"
          cosca knowledge search "$(basename $file)" --limit 3 --format json >> .cosca/reports/knowledge-results.json
        fi
      done < .cosca/reports/changed-files.txt
      
      echo "Knowledge review complete"
    outputs:
      changed_files: ".cosca/reports/changed-files.txt"
      knowledge_hits: ".cosca/reports/knowledge-results.json"

  - id: "report"
    name: "Generate Report"
    description: "Combine analysis and knowledge results into a review report"
    timeout: "30s"
    depends_on: ["review"]
    command: |
      #!/bin/bash
      echo "Generating review report..."
      
      REPORT_FILE=".cosca/reports/review-$(date +%Y%m%d-%H%M%S).md"
      
      cat > "$REPORT_FILE" << 'REPORT_HEADER'
      # Code Review Report
      
      **Branch:** {{ .branch }}
      **Base:** {{ .base_branch }}
      **Date:** {{ .timestamp }}
      
      ---
      
      ## Summary
      
      REPORT_HEADER
      
      # Add issue summary
      if [ -f ".cosca/reports/analysis-results.json" ]; then
        echo "" >> "$REPORT_FILE"
        echo "### Static Analysis Results" >> "$REPORT_FILE"
        python3 -c "
        import json
        with open('.cosca/reports/analysis-results.json') as f:
            data = json.load(f)
        print(f'**Total Issues:** {data[\"total\"]}', file=open('$REPORT_FILE', 'a'))
        print('', file=open('$REPORT_FILE', 'a'))
        for issue in data['issues']:
            print(f'- **{issue[\"tool\"]}**: {issue[\"file\"]}:{issue[\"line\"]} — {issue[\"message\"]}', file=open('$REPORT_FILE', 'a'))
        "
      fi
      
      # Add knowledge findings
      if [ -f ".cosca/reports/knowledge-results.json" ]; then
        echo "" >> "$REPORT_FILE"
        echo "### Knowledge Base Findings" >> "$REPORT_FILE"
        echo "Related documentation and patterns found in knowledge base." >> "$REPORT_FILE"
      fi
      
      echo "" >> "$REPORT_FILE"
      echo "---" >> "$REPORT_FILE"
      echo "_Generated by Cosca Workflow Engine_" >> "$REPORT_FILE"
      
      # Store report path for next step
      echo "$REPORT_FILE" > .cosca/reports/latest-report.txt
      
      echo "Report generated: $REPORT_FILE"
    outputs:
      report_path: ".cosca/reports/latest-report.txt"

  - id: "store"
    name: "Store in Memory"
    description: "Store review summary in Cosca memory for future reference"
    timeout: "10s"
    depends_on: ["report"]
    command: |
      #!/bin/bash
      echo "Storing review in Cosca memory..."
      
      REPORT_FILE=$(cat .cosca/reports/latest-report.txt)
      
      # Create a summary for memory storage
      cosca memory store \
        --key "code-review-{{ .branch }}" \
        --type "project" \
        --content "Code review completed for branch {{ .branch }} against {{ .base_branch }}. Report: $REPORT_FILE" \
        --tags "code-review,{{ .branch }},automated"
      
      echo "Review stored in memory"
    outputs:
      memory_key: "code-review-{{ .branch }}"

# Error handling
on_error:
  - step: "analyze"
    action: "retry"
    max_retries: 2
  - step: "review"
    action: "skip"
    message: "Knowledge review unavailable, continuing with static analysis only"
  - step: "report"
    action: "fail"
    message: "Failed to generate report"
  - step: "store"
    action: "warn"
    message: "Review completed but could not be stored in memory"

# Validation rules
validation:
  - name: "branch_exists"
    description: "Ensure the branch exists"
    command: "git rev-parse --verify origin/{{ .branch }}"
  - name: "no_uncommitted"
    description: "Ensure working directory is clean"
    command: "git diff --quiet HEAD"

# Hooks
hooks:
  on_step_start:
    - command: "echo 'Starting step {{ .step_id }}'"
  on_step_complete:
    - command: "echo 'Completed step {{ .step_id }}'"
  on_workflow_complete:
    - command: "cosca memory store --key 'workflow-run-{{ .timestamp }}' --content 'Code review workflow completed for {{ .branch }}'"
```

---

## Step 3: Register the Workflow

```bash
# Register the workflow with Cosca
cosca workflow register .cosca/workflows/code-review.yaml

# Verify registration
cosca workflow list
```

**Expected output:**

```
WORKFLOW          VERSION  STATUS   STEPS
code-review       1.0.0    ready    4
```

```bash
# View workflow details
cosca workflow info code-review
```

**Expected output:**

```
Workflow: code-review
  Version: 1.0.0
  Status: ready
  Steps:
    1. analyze       — Static Analysis (60s timeout)
    2. review        — Knowledge-Based Review (depends on: analyze)
    3. report        — Generate Report (depends on: review)
    4. store         — Store in Memory (depends on: report)
  Inputs:
    - branch (required)
    - base_branch (optional, default: main)
    - severity (optional, default: all)
  Hooks: 3 registered
```

---

## Step 4: Run the Workflow

### Basic Run

```bash
# Run the workflow for a feature branch
cosca workflow run code-review \
  --input branch="feature/add-auth" \
  --input base_branch="main"
```

### Run with Custom Parameters

```bash
# Run with custom severity filter
cosca workflow run code-review \
  --input branch="feature/add-auth" \
  --input base_branch="develop" \
  --input severity="critical"
```

### Run Output

```
╭─────────────────────────────────────────────────────╮
│         Running Workflow: code-review                 │
├─────────────────────────────────────────────────────┤
│ Inputs:                                               │
│   branch: feature/add-auth                            │
│   base_branch: main                                   │
│   severity: all                                       │
├─────────────────────────────────────────────────────┤
│ Step 1/4: Static Analysis                             │
│   → Running go vet...                                 │
│   → Found 3 issue(s)                                  │
│   ✅ Completed (12.3s)                                │
├─────────────────────────────────────────────────────┤
│ Step 2/4: Knowledge-Based Review                      │
│   → Searching knowledge base for changed files...     │
│   → Found 5 related documents                         │
│   ✅ Completed (45.1s)                                │
├─────────────────────────────────────────────────────┤
│ Step 3/4: Generate Report                             │
│   → Generating review report...                       │
│   ✅ Completed (1.2s)                                 │
├─────────────────────────────────────────────────────┤
│ Step 4/4: Store in Memory                             │
│   → Storing review summary...                         │
│   ✅ Completed (0.8s)                                 │
├─────────────────────────────────────────────────────┤
│ Workflow Complete                                      │
│   Duration: 59.4s                                     │
│   Status: completed                                   │
│   Outputs:                                            │
│     review_report: .cosca/reports/review-20260723.md    │
│     review_summary: code-review-feature/add-auth      │
╰─────────────────────────────────────────────────────╯
```

---

## Step 5: View Results

### Review Report

```bash
cat .cosca/reports/review-20260723-103022.md
```

**Expected output:**

```markdown
# Code Review Report

**Branch:** feature/add-auth
**Base:** main
**Date:** 2026-07-23T10:30:22Z

---

## Summary

### Static Analysis Results

**Total Issues:** 3

- **go vet**: internal/auth/handler.go:42 — unreachable code
- **go vet**: internal/auth/middleware.go:88 — ineffective assignment
- **eslint**: src/components/Login.tsx:15 — 'unusedVar' is defined but never used

### Knowledge Base Findings

Related documentation and patterns found in knowledge base.

---

_Generated by Cosca Workflow Engine_
```

### Memory Record

```bash
cosca memory get "code-review-feature/add-auth"
```

**Expected output:**

```json
{
  "key": "code-review-feature/add-auth",
  "content": "Code review completed for branch feature/add-auth against main. Report: .cosca/reports/review-20260723-103022.md",
  "type": "project",
  "tags": ["code-review", "feature/add-auth", "automated"],
  "timestamp": "2026-07-23T10:30:23Z"
}
```

---

## Workflow Status Management

```bash
# List all workflow runs
cosca workflow list-runs

# Check status of a specific run
cosca workflow status <run-id>

# Cancel a running workflow
cosca workflow cancel <run-id>

# Rerun a failed workflow
cosca workflow rerun <run-id>

# View workflow logs
cosca workflow logs <run-id>
```

---

## Advanced Workflow Features

### Conditional Steps

```yaml
steps:
  - id: "deploy-review"
    name: "Deploy Review"
    description: "Additional review for deployment branches"
    condition: "startsWith(.branch, 'release/')"
    command: |
      echo "Running deployment review for release branch..."
      cosca knowledge search "deployment checklist" --limit 5
```

### Parallel Steps

```yaml
steps:
  - id: "lint-go"
    name: "Lint Go Code"
    parallel_group: "lint"

  - id: "lint-ts"
    name: "Lint TypeScript Code"
    parallel_group: "lint"

  - id: "merge-results"
    name: "Merge Lint Results"
    depends_on: ["lint-go", "lint-ts"]  # Waits for all parallel steps
```

### User Input Steps

```yaml
steps:
  - id: "approve"
    name: "Request Approval"
    type: "input"
    prompt: |
      Approved the code review for branch {{ .branch }}?
    options:
      - value: "approve"
        label: "Approve"
      - value: "request-changes"
        label: "Request Changes"
      - value: "abort"
        label: "Abort Workflow"
```

---

## Workflow Templates

Cosca includes built-in workflow templates:

```bash
# List available templates
cosca workflow templates

# Initialize from a template
cosca workflow init code-review
cosca workflow init deployment
cosca workflow init documentation-sync
```

---

**Related**: [Workflow Overview](../../docs/runtime/overview.md) | [Runtime Configuration](../../docs/runtime/configuration.md) | [Plugin System](../../docs/plugins/overview.md)
