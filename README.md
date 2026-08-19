# AXON

> **Autonomous eXecution & Orchestration Network**

**AXON is an autonomous software engineering agent built around a programmable agent runtime.**

AXON is designed to move beyond simple "LLM + tools" workflows and provide a unified runtime for reasoning, planning, tool execution, context management, and autonomous task completion.

## Why AXON?

Modern coding agents are increasingly capable, but many of them are still fundamentally implemented as a loop:

```text
Prompt
  ↓
LLM
  ↓
Tool Call
  ↓
Tool Result
  ↓
LLM
  ↓
...
```

AXON treats this loop as a **runtime problem**, rather than merely a prompting problem.

The goal is to build a system where an agent can:

```text
Understand
   ↓
Reason
   ↓
Plan
   ↓
Act
   ↓
Observe
   ↓
Adapt
   ↓
Execute
   ↺
```

The agent should be able to determine **what needs to be done, how it should be done, which capabilities are required, and when the task is actually complete**.

---

## Architecture

At its core, AXON is an execution and orchestration layer connecting models, tools, context, and the environment.

```text
                         User Intent
                              │
                              ▼
                     ┌─────────────────┐
                     │     Agent       │
                     │                 │
                     │ Reasoning       │
                     │ Planning        │
                     │ Decision Making │
                     └────────┬────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │  AXON Runtime     │
                    │                   │
                    │ Execution         │
                    │ Orchestration     │
                    │ Context           │
                    │ State             │
                    │ Lifecycle         │
                    └─────────┬─────────┘
                              │
             ┌────────────────┼────────────────┐
             │                │                │
             ▼                ▼                ▼
        ┌─────────┐      ┌─────────┐      ┌──────────┐
        │  Tools  │      │  MCP    │      │ Subagent │
        └─────────┘      └─────────┘      └──────────┘
             │                │                │
             └────────────────┼────────────────┘
                              ▼
                         Environment
                              │
                              ▼
                          Observation
                              │
                              └──────────────► Agent
```

The runtime is responsible for turning model decisions into reliable execution.

---

## Core Concepts

### Agent

The agent is responsible for reasoning and decision making.

It determines:

* What is the user's actual intent?
* What should happen next?
* Which tool or capability should be used?
* Whether more information is required?
* Whether the task has been completed?

### Runtime

The runtime provides the execution environment for the agent.

It manages:

* Agent lifecycle
* Execution loops
* Context
* State
* Tool invocation
* Events
* Errors
* Interruptions
* Sub-agents
* Persistence and resume

The runtime is deliberately separated from the model.

**Models reason. The runtime executes.**

### Tools

Tools are the agent's interface to the outside world.

Examples include:

* File system
* Shell
* Git
* Search
* Browser
* MCP
* Build systems
* Test runners
* Custom developer tools

AXON treats tools as first-class runtime capabilities rather than simply function definitions attached to a prompt.

### Context

Context is more than a conversation history.

AXON aims to provide structured context management across:

* User requests
* Agent reasoning
* Tool calls
* Tool results
* Files
* Repository state
* Execution state
* Previous actions
* Runtime events

### Orchestration

Complex software engineering tasks rarely require a single linear chain of actions.

AXON is designed to orchestrate:

```text
Agent
 ├── Tool
 ├── Tool
 ├── Sub-agent
 │    ├── Tool
 │    └── Tool
 ├── MCP
 └── Tool
```

The orchestration layer provides a foundation for parallel execution, delegation, recovery, and long-running tasks.

---

## Software Engineering

AXON's first and primary application is autonomous software engineering.

Given a task such as:

```text
Implement user authentication.

Requirements:
- Add JWT authentication
- Add login API
- Add refresh token support
- Add integration tests
- Update documentation
```

AXON should be able to independently:

```text
Analyze repository
      ↓
Understand architecture
      ↓
Plan implementation
      ↓
Inspect relevant files
      ↓
Modify code
      ↓
Run tests
      ↓
Observe failures
      ↓
Diagnose
      ↓
Fix
      ↓
Run tests again
      ↓
Verify completion
```

The objective is not to generate code.

The objective is to **complete the engineering task**.

---

## Design Principles

### 1. Runtime over Prompt

Prompts are important, but reliable agents cannot be built entirely inside prompts.

AXON moves critical agent behavior into explicit runtime primitives.

### 2. Model Agnostic

AXON should not depend on a single model provider.

Models should be replaceable without rewriting the runtime.

### 3. Tool First

The agent should interact with the real environment through well-defined capabilities.

### 4. Observable

Every important runtime transition should be observable.

```text
AgentStarted
  ↓
ReasoningStarted
  ↓
ToolCallStarted
  ↓
ToolCallCompleted
  ↓
ModelResponse
  ↓
AgentCompleted
```

Observability is a core runtime capability, not an afterthought.

### 5. Interruptible

Long-running autonomous tasks must be interruptible.

The runtime should support:

* Cancel
* Pause
* Resume
* Retry
* Recover

### 6. Extensible

AXON should make it possible to add new:

* Models
* Tools
* Providers
* Skills
* Agents
* Runtime components
* Orchestration strategies

without changing the core architecture.

---

## Project Status

> 🚧 **Early Development**

AXON is currently under active development.

The architecture and APIs are expected to evolve rapidly.

The current focus is building the underlying agent runtime and execution model before expanding the higher-level coding-agent experience.

---

## Roadmap

### Runtime

* [ ] Agent lifecycle
* [ ] Execution loop
* [ ] Event system
* [ ] Context management
* [ ] State management
* [ ] Interrupt / resume
* [ ] Error recovery
* [ ] Persistence

### Model

* [ ] OpenAI Responses API
* [ ] Anthropic Messages API
* [ ] Gemini
* [ ] Model provider abstraction
* [ ] Streaming

### Tools

* [ ] File system
* [ ] Shell
* [ ] Git
* [ ] MCP
* [ ] Tool permissions
* [ ] Tool lifecycle

### Agent

* [ ] Planning
* [ ] Autonomous execution
* [ ] Task decomposition
* [ ] Sub-agents
* [ ] Agent delegation
* [ ] Long-running tasks

### Developer Experience

* [ ] Interactive CLI
* [ ] TUI
* [ ] Session management
* [ ] Session resume
* [ ] Debugging
* [ ] Runtime tracing

---

## Philosophy

AXON is built around a simple idea:

> **An intelligent model is not an agent.**

A model can reason.

An agent must **reason, act, observe, adapt, and continue**.

And a reliable agent needs a runtime capable of coordinating that entire process.

AXON aims to provide that runtime.

```text
          Reason
             │
             ▼
          Decide
             │
             ▼
           Act
             │
             ▼
         Observe
             │
             ▼
          Adapt
             │
             └───────────────┐
                             ▼
                           Act
```

**AXON is the execution and orchestration layer that turns reasoning into autonomous action.**

---

## Name

**AXON** stands for:

> **Autonomous eXecution & Orchestration Network**

The name is inspired by the biological **axon** — the part of a neuron responsible for transmitting signals.

In the same spirit, AXON connects:

```text
Reasoning
    ↕
Context
    ↕
Orchestration
    ↕
Tools
    ↕
Execution
    ↕
Environment
```

It is the communication and execution layer between an agent's intelligence and the world it operates in.

---

## License

TBD.
