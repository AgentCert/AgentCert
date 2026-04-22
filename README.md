# AgentCert

### AI Agent Benchmarking Platform for Chaos Engineering

## What is AgentCert?

**AgentCert** is a groundbreaking platform that brings **AI Agent Benchmarking** capabilities to the world of **Chaos Engineering**. Built on the proven foundation of [LitmusChaos](docs/litmus-core/README.md), AgentCert introduces a comprehensive framework for evaluating how AI agents perform when your Kubernetes infrastructure experiences failures.

In today's AI-driven operations landscape, autonomous agents are increasingly responsible for detecting, diagnosing, and remediating infrastructure issues. But how do you know if your AI agent will respond effectively when a critical pod crashes or network latency spikes? **AgentCert answers this question.**

## Why AgentCert?

### The Challenge

Modern cloud-native applications rely on AI agents for:

-   Automated incident detection and response
-   Intelligent resource optimization
-   Predictive failure prevention
-   Self-healing infrastructure management

But without proper validation, you're trusting these agents to handle production incidents **blindly**. Traditional testing doesn't expose agents to the chaotic, unpredictable nature of real-world failures.

### The Solution

AgentCert creates **controlled chaos experiments** in your Kubernetes clusters and evaluates how your AI agents respond. It measures:

-   ⏱️ **Time to Detect (TTD)**: How quickly does your agent notice the problem?
-   🔧 **Time to Remediate (TTR)**: How fast can it fix the issue?
-   ✅ **Success Rate**: Does the agent actually resolve the fault?
-   🎯 **Decision Quality**: Are the agent's actions logical and safe?
-   💡 **Resource Efficiency**: Does the agent consume excessive resources?

By injecting faults like pod crashes, network failures, resource exhaustion, etc. AgentCert validates your agent's resilience **before production incidents happen**.

## How It Works

AgentCert combines three powerful technologies:

### 1. LitmusChaos Foundation

Built on the proven LitmusChaos platform, AgentCert inherits:

-   **Battle-tested fault injection** for Kubernetes, cloud services, and infrastructure
-   **Workflow orchestration** via Argo Workflows for complex multi-stage scenarios
-   **Community-driven chaos hub** with pre-built experiment templates
-   **CNCF-backed reliability** and enterprise-grade security

### 2. NVIDIA NeMo Agent Toolkit (NAT)

AgentCert uses NAT as the standard evaluation framework, providing:

-   **Task-Agent-Evaluator model** for structured benchmarking
-   **Built-in evaluators** (RAGAS for LLM reasoning, Trajectory for action sequences)
-   **Custom chaos evaluators** for TTD, TTR, and remediation quality
-   **Production-ready runtime** for agent execution and monitoring

### 3. Langfuse Observability

All agent behavior is tracked through Langfuse, offering:

-   **Complete trace visibility** of every agent decision and action
-   **Real-time metrics dashboard** for performance monitoring
-   **Historical analysis** to compare agent versions and configurations

## Quick Example: Pod Crash Benchmark

Here's what happens when you benchmark an AI agent's response to a pod crash:

```
1. AgentCert injects fault → Pod crashes in target cluster2. NAT Runtime monitors → AI agent queries Kubernetes API3. Agent detects crash → Decides to restart the pod4. Agent remediates → Executes kubectl commands5. System recovers → Pod returns to healthy state6. NAT evaluates → Calculates TTD, TTR, Success Percentage, etc.7. Langfuse persists → Trace, metrics, and scores stored permanently8. Dashboard updates → Real-time performance visualization
```

## Key Features

### AI Agent Management

-   **Centralized Registry**: Register agents with metadata, endpoints, and capabilities
-   **Multi-Agent Support**: Benchmark multiple agents simultaneously for comparison
-   **Version Tracking**: Compare different versions of the same agent
-   **Credential Management**: Secure storage of API keys and authentication tokens

### Benchmark Scenarios

-   **Pre-built Templates**: Pod crashes, network latency, disk pressure, resource exhaustion
-   **Custom Scenarios**: Define your own fault patterns and expected behaviors
-   **Multi-Fault Sequences**: Test agent resilience against cascading failures
-   **Difficulty Levels**: Progressive complexity from basic to advanced scenarios

### Evaluation & Analytics

-   **Automated Scoring**: NAT evaluators measure TTD, TTR, and success rates
-   **RAGAS Evaluation**: Validates LLM-based reasoning quality for intelligent agents
-   **Trajectory Analysis**: Ensures agents follow safe, optimal action sequences
-   **Comparative Reports**: Side-by-side performance of multiple agents or versions

#### NAT Built-in Evaluators

AgentCert leverages industry-standard evaluators from NVIDIA's NeMo Agent Toolkit:

Evaluator

Purpose

Why It Matters for Chaos Engineering

**RAGAS**

Evaluates quality of LLM-generated responses based on retrieved context

Many AI agents use RAG patterns to analyze Kubernetes state (pod logs, events, metrics). RAGAS validates whether agent reasoning is grounded in actual cluster data, not hallucinations.

**Trajectory Evaluator**

Evaluates the sequence of actions taken by an agent

Critical for chaos scenarios where action sequence matters (e.g., must drain node before terminating pods). Ensures agents don't skip steps that could cause cascading failures.

#### Custom Chaos Evaluators

Built on top of NAT's framework specifically for chaos resilience:

-   **TTD (Time to Detect)**: Measures time from fault injection to first agent action indicating detection
-   **TTR (Time to Remediate)**: Measures time from detection to complete system recovery
-   **Remediation Success**: Validates that agent actions actually resolved the fault
-   **Resource Efficiency**: Measures CPU/memory overhead during agent operation
-   **Decision Quality**: Uses RAGAS to assess quality of LLM-based agent reasoning

### Standards-Based Integration

-   **NAT Framework**: Industry-standard agent evaluation toolkit from NVIDIA
-   **Langfuse Platform**: Production-ready LLM observability without custom infrastructure
-   **ChaosHub Compatibility**: Reuse existing chaos experiments from the community
-   **Argo Workflows**: Proven orchestration for complex benchmark pipelines

## Getting Started

### Prerequisites

-   Kubernetes cluster (v1.20+)
-   kubectl configured with cluster access
-   Helm 3.x (for installation)
-   AI agent with REST API or SDK integration

### Installation

*To Do - Add commands*

### Register Your First Agent

```bash
# Using LitmusCtl CLI*To Do - Add commands*# Or via Web UI: http://<agentcert-url>/agents/register
```

### Run Your First Benchmark

```bash
# Create a benchmark project*To Do - Add commands*# Start the benchmark*To Do - Add commands*# View real-time results*To Do - Add commands*
```

### View Results in Langfuse Dashboard

Access your Langfuse dashboard to see:

-   Complete trace of agent actions and decisions
-   TTD and TTR metrics for each benchmark run
-   Comparative performance across multiple runs

## Use Cases

### For AI Agent Developers

-   **Validate agent logic** before deploying to production
-   **Benchmark performance** against industry baselines
-   **Compare agent versions** to measure improvements
-   **Test edge cases** that are hard to reproduce manually

### For Platform Engineering Teams

-   **Certify agent reliability** for production readiness
-   **Establish SLAs** for incident response (e.g., TTD < 30s, TTR < 2m)
-   **Regression testing** after infrastructure changes

### For SREs & Operations

-   **Validate incident response** automation before outages occur
-   **Test agent behavior** under multi-fault scenarios
-   **Evaluate decision quality** for safety-critical actions
-   **Monitor agent performance** trends over time

### For Researchers & Academia

-   **Benchmark new agent algorithms** against standard scenarios
-   **Publish reproducible results** using community scenarios
-   **Compare different approaches** (rule-based vs. LLM-based agents)
-   **Contribute evaluation metrics** to the open-source community

## What Makes AgentCert Different?

Traditional Testing

AgentCert Approach

Synthetic test data

Real Kubernetes fault injection

Isolated unit tests

End-to-end chaos scenarios

Manual verification

Automated scoring with NAT

No production simulation

Controlled production-like failures

Basic pass/fail

Quantified metrics (TTD, TTR, quality)

## License

AgentCert is licensed under the **Apache License, Version 2.0**. See [LICENSE](./LICENSE) for the full license text.

This project builds on LitmusChaos, which is also Apache 2.0 licensed. Some integrated components (NAT, Langfuse) may be governed by different licenses - please refer to their respective documentation.

## Acknowledgments

AgentCert stands on the shoulders of giants:

-   [LitmusChaos](https://litmuschaos.io): For the robust chaos engineering foundation
-   [NVIDIA NeMo Agent Toolkit](https://developer.nvidia.com/nemo-agent-toolkit): For the NAT framework and agent evaluation standards
-   [Langfuse](https://langfuse.com): For the production-ready LLM observability platform
-   [Kubernetes](https://kubernetes.io/): For the orchestration platform that powers modern infrastructure
-   [Argo Workflows](https://argoproj.github.io/workflows) - Kubernetes-native Workflow Engine