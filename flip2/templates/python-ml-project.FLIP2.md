# FLIP2.md - Python ML Project Configuration

**Project:** Python ML Project
**Version:** 1.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-01

---

## Overview

This FLIP2.md configuration optimizes agent routing for machine learning projects. Specialized roles handle data analysis, model training, evaluation, and deployment with intelligent cost-based routing between Gemini (data exploration) and Opus (complex training).

---

## Agents

Define custom agent roles for ML development lifecycle.

### Agent Role: Data Scientist
- **ID Pattern:** `data-scientist-*`
- **Model:** gemini
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 4
- **Escalation Required For:** `access-secrets`
- **Cost Budget (USD/hour):** 3.50
- **Description:** Explores data, performs EDA, identifies patterns, and generates hypotheses. Gemini excels at bulk data analysis and exploratory workflows at lower cost.

### Agent Role: ML Engineer
- **ID Pattern:** `ml-engineer-*`
- **Model:** opus
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 6.50
- **Description:** Designs architectures, implements training pipelines, handles complex model development. Opus's superior reasoning essential for ML engineering decisions.

### Agent Role: Model Evaluator
- **ID Pattern:** `evaluator-*`
- **Model:** gemini
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 6
- **Escalation Required For:** `access-secrets`
- **Cost Budget (USD/hour):** 2.50
- **Description:** Evaluates model performance, runs validation tests, analyzes metrics. Cost-optimized for systematic evaluation and metric analysis.

### Agent Role: Research Lead
- **ID Pattern:** `research-lead-*`
- **Model:** opus
- **Capabilities:** `spawn-workers, read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks, escalate`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 7.00
- **Description:** Leads research initiatives, coordinates team, makes architectural decisions. Uses Opus for complex reasoning about ML approaches and novel solutions.

### Agent Role: MLOps Specialist
- **ID Pattern:** `mlops-*`
- **Model:** gemini
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 3.00
- **Description:** Manages deployment pipelines, containerization, monitoring, and production ML systems. Gemini effective for infrastructure and operational tasks.

---

## Commands

Register project-specific slash commands for ML workflows.

### Command: /explore-data
- **Aliases:** `explore, eda, analyze-data`
- **Handler:** `data-scientist-worker`
- **Args:** `<dataset-path> [--output=report|notebook] [--depth=quick|thorough]`
- **Description:** Run exploratory data analysis, generate statistics, visualizations, and insights
- **Requires Approval:** no
- **Allowed Roles:** `data-scientist, research-lead, coordinator`

### Command: /train-model
- **Aliases:** `train, start-training, build-model`
- **Handler:** `ml-engineer-worker`
- **Args:** `<model-type> [--epochs=N] [--batch-size=N] [--hyperparams=FILE]`
- **Description:** Train machine learning model with specified architecture and hyperparameters
- **Requires Approval:** no
- **Allowed Roles:** `ml-engineer, coordinator`

### Command: /evaluate-model
- **Aliases:** `evaluate, test-model, validate`
- **Handler:** `evaluator-worker`
- **Args:** `<model-path> <test-dataset> [--metrics=accuracy|f1|roc|all] [--threshold=VALUE]`
- **Description:** Evaluate model performance on test dataset with comprehensive metrics
- **Requires Approval:** no
- **Allowed Roles:** `evaluator, research-lead, coordinator`

### Command: /deploy
- **Aliases:** `deploy, push, release-model`
- **Handler:** `mlops-worker`
- **Args:** `<model-version> <environment> [--dry-run] [--rollback-on-fail]`
- **Description:** Deploy model to staging or production with monitoring setup
- **Requires Approval:** yes
- **Allowed Roles:** `mlops, research-lead, coordinator`

### Command: /research
- **Aliases:** `investigate, research-topic, explore-approach`
- **Handler:** `research-lead-worker`
- **Args:** `<research-question> [--scope=narrow|broad] [--iterations=N]`
- **Description:** Investigate ML approach, research state-of-the-art, propose solutions
- **Requires Approval:** no
- **Allowed Roles:** `research-lead, coordinator`

### Command: /pipeline
- **Aliases:** `pipeline, etl, preprocess`
- **Handler:** `mlops-worker`
- **Args:** `<pipeline-name> [--validate] [--profile]`
- **Description:** Run data preprocessing and feature engineering pipeline
- **Requires Approval:** no
- **Allowed Roles:** `mlops, data-scientist, coordinator`

---

## Routing

Define intelligent routing based on task type, complexity, and cost optimization.

### Route: Exploratory Data Analysis
- **When:** `task.type == "eda" || task.type == "exploration"`
- **Route To:** `gemini`
- **Reason:** Gemini excellent for bulk data analysis, pattern discovery, and exploratory workflows
- **Cost Impact:** `-0.45`

### Route: Complex Model Training
- **When:** `task.type == "training" && (task.complexity >= 7 || task.requires_advanced_ml == true)`
- **Route To:** `opus`
- **Reason:** Complex training architectures, novel approaches, optimization require Opus's superior reasoning
- **Cost Impact:** `+0.85`

### Route: Routine Model Training
- **When:** `task.type == "training" && task.complexity < 7`
- **Route To:** `gemini`
- **Reason:** Standard training scripts and hyperparameter tuning are cost-effective with Gemini
- **Cost Impact:** `-0.40`

### Route: Model Evaluation & Validation
- **When:** `task.type == "evaluation" || task.type == "validation"`
- **Route To:** `gemini`
- **Reason:** Systematic evaluation and metric analysis are routine; Gemini provides excellent cost efficiency
- **Cost Impact:** `-0.50`

### Route: Research & Innovation
- **When:** `task.type == "research" && (task.requires_novel_approach == true || task.complexity >= 8)`
- **Route To:** `opus`
- **Reason:** Novel ML approaches and research decisions need Opus's superior reasoning capabilities
- **Cost Impact:** `+0.90`

### Route: Deployment & MLOps
- **When:** `task.type == "deployment" || task.type == "mlops"`
- **Route To:** `gemini`
- **Reason:** Deployment pipelines and infrastructure tasks are well-handled by Gemini
- **Cost Impact:** `-0.35`

### Route: High-Priority Research
- **When:** `task.priority == "high" && task.type == "research"`
- **Route To:** `opus`
- **Reason:** Critical research questions demand Opus's analytical capabilities
- **Cost Impact:** `+0.95`

### Route: Data Preprocessing
- **When:** `task.type == "preprocessing" || task.type == "feature-engineering"`
- **Route To:** `gemini`
- **Reason:** Systematic preprocessing and feature engineering are cost-effective with Gemini
- **Cost Impact:** `-0.40`

---

## Context

Specify files to auto-load when spawning agents for this ML project.

### Auto-Load Files
- `./README.md` - Project overview, setup instructions, and quick start (weight: high)
- `./requirements.txt` - Python dependencies and versions (weight: high)
- `./notebooks/*.ipynb` - Jupyter notebooks with analysis and experiments (weight: high)
- `./src/models/*.py` - Model architectures and implementations (weight: high)
- `./docs/DATA_SCHEMA.md` - Dataset structure, features, and transformations (weight: high)
- `./docs/ARCHITECTURE.md` - ML pipeline design and component relationships (weight: high)
- `./src/preprocessing/*.py` - Data preprocessing and feature engineering (weight: medium)
- `./docs/EXPERIMENTS.md` - Experiment tracking, results, and findings (weight: medium)
- `./src/training/config/*.yaml` - Training configurations and hyperparameters (weight: medium)
- `./scripts/deploy.sh` - Model deployment automation (weight: medium)
- `.env.example` - Environment variables template (weight: low)
- `./docs/TROUBLESHOOTING.md` - Common issues and solutions (weight: low)

---

## Example Workflows

### Workflow 1: Data Exploration & Analysis
1. User runs: `/explore-data ./data/raw_dataset.csv --depth=thorough`
2. Routes to `data-scientist-worker` handler
3. Agent loads: requirements.txt, notebooks/**, DATA_SCHEMA.md
4. Gemini explores data, generates visualizations, identifies patterns
5. Returns EDA report with statistical analysis and insights
6. Cost optimized: Gemini saves ~45% vs Opus

### Workflow 2: Model Training & Evaluation Pipeline
1. User runs: `/train-model transformer --epochs=10 --batch-size=32 --hyperparams=./config/prod.yaml`
2. Routes based on complexity:
   - Simple training (complexity < 7) → Gemini (-40% cost)
   - Complex training (complexity >= 7) → Opus (+85% cost)
3. Agent loads: requirements.txt, models/**, training/config/**
4. Training executes with specified hyperparameters
5. User runs: `/evaluate-model ./models/v1.pth ./data/test.csv --metrics=all`
6. Routes to `evaluator-worker` → Gemini for cost efficiency
7. Comprehensive evaluation report with F1, ROC, accuracy, precision/recall

### Workflow 3: Research Initiative
1. User runs: `/research "How to improve model accuracy for imbalanced data" --scope=broad --iterations=3`
2. Routes to `research-lead-worker` handler
3. Agent loads all context files
4. Opus analyzes state-of-the-art, proposes techniques
5. Research lead spawns `data-scientist` and `ml-engineer` workers
6. Team investigates together, reports findings
7. High-value research decisions made with Opus's reasoning

### Workflow 4: Full ML Deployment Pipeline
1. User runs: `/pipeline feature-engineering-v2 --validate --profile`
2. Routes to `mlops-worker` → Gemini for infrastructure efficiency
3. Preprocessing pipeline executes with profiling
4. User runs: `/train-model production-model --epochs=50`
5. Routes based on complexity (likely Opus for production models)
6. After training succeeds: `/evaluate-model ./models/prod.pth ./data/test.csv`
7. Routes to evaluator → Gemini for metrics
8. User runs: `/deploy production-v2.1 production --dry-run`
9. Requires approval, then actual deployment executes
10. MLOps sets up monitoring and alerts

---

## Configuration Notes

### Model Selection Strategy
- **Gemini:** EDA, evaluation, preprocessing, deployment - systematic work at scale
- **Opus:** Training logic, research, complex ML architecture decisions
- **Research lead (Opus):** Coordinates team, makes novel approach decisions

### Cost Optimization
- **Data exploration:** -45% (Gemini handles bulk analysis well)
- **Standard training:** -40% (Gemini can handle routine training)
- **Complex training:** +85% (Opus needed for advanced architectures)
- **Evaluation:** -50% (Systematic metric analysis is cost-effective)
- **Deployment:** -35% (Infrastructure automation is Gemini-suitable)

### Gemini's ML Strengths
- Processing large datasets and exploratory analysis
- Running systematic evaluation frameworks
- Infrastructure and deployment tasks
- Hyperparameter tuning scripts

### Opus's ML Strengths
- Complex training architecture design
- Novel ML approaches and research
- Advanced optimization strategies
- Architectural decision-making

### Capability Restrictions
- Only `research-lead` can spawn workers (coordination)
- `ml-engineer` for production training decisions
- `evaluator` for final model validation

### Context Priority
1. **High:** requirements.txt, notebooks, models, data schema (loaded first)
2. **Medium:** Training configs, preprocessing, experiments (loaded second)
3. **Low:** Templates, environment examples (loaded last)

---

## Validation

Before using this configuration in production:

```bash
# Validate syntax and schema
flip2 validate --config ./python-ml-project.FLIP2.md

# Validate specific sections
flip2 validate --config ./python-ml-project.FLIP2.md --section agents
flip2 validate --config ./python-ml-project.FLIP2.md --section commands
flip2 validate --config ./python-ml-project.FLIP2.md --section routing
flip2 validate --config ./python-ml-project.FLIP2.md --section context
```

---

## Customization Guide

1. **Add custom model types:** Update `/train-model` command with new model architectures
2. **Extend metrics:** Add new evaluation metrics to `/evaluate-model` command
3. **Add preprocessing steps:** Define custom preprocessing pipelines in context
4. **Adjust complexity thresholds:** Modify routing based on actual model complexity
5. **Add experiment tracking:** Integrate with MLflow, Weights & Biases, or similar

---

**Status:** Production Ready
**Created for:** CFG-007 - FLIP2 Template Generation
**Template Use:** Copy as `FLIP2.md` to Python ML projects
