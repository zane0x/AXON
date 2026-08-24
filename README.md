# AXON 🚀

[![Go Report Card](https://goreportcard.com/badge/github.com/zane0x/AXON)](https://goreportcard.com/report/github.com/zane0x/AXON)
[![License](https://img.shields.io/github/license/zane0x/AXON)](https://github.com/zane0x/AXON/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zane0x/AXON)](https://github.com/zane0x/AXON)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://github.com/zane0x/AXON/pulls)

> **Autonomous eXecution & Orchestration Network**

**AXON** is a high-performance, single-binary **autonomous AI software engineering agent** and runtime built in Go. 

Designed to operate directly in your terminal, AXON goes beyond simple "LLM + wrapper" setups to provide a stateful runtime for planning, tool execution, session persistence, and self-correcting development loops. Think of it as a blazing-fast, lightweight, and model-agnostic alternative to tools like **Claude Code**, **Aider**, and **Mentat**.

---

## ⚡ Quick Demo

Here is how AXON works in action:

```text
$ export LLM_END_PROT="https://api.openai.com/v1"
$ export LLM_TOKEN="your-api-key"
$ axon --model gpt-4o

[session] new session 20260824_220130
[session] file: /home/user/.axon/sessions/20260824_220130.jsonl

> Find the bug in the auth validation and fix it.

🤖 Thinking...
🔧 Calling tool [read] {"path": "auth/validate.go"}
📝 Tool Result: (Read 1,420 bytes)

🤖 Thinking...
🔧 Calling tool [edit] {
      "path": "auth/validate.go",
      "old_text": "if token == \"\" {\n\treturn true\n}",
      "new_text": "if token == \"\" {\n\treturn false\n}"
   }
📝 Tool Result: Successfully edited auth/validate.go

🤖 Thinking...
🔧 Calling tool [bash] {"command": "go test ./auth/..."}
📝 Tool Result: PASS ok auth/validation [0.12s]

🤖 Done! I found the bug where empty tokens were mistakenly validated as true, corrected the check, and successfully verified it with the unit tests.
```

---

## 🔥 Key Highlights

* 🏎️ **Single-Binary & Blazing Fast**: Built in Go with zero external runtime dependencies (no Node.js, no Python startup lags).
* 🧩 **Model Agnostic**: Works with any OpenAI-compatible API endpoint (OpenAI, Anthropic via proxy, Gemini, DeepSeek, Ollama, vLLM).
* 🛠️ **Surgical File Edits**: Uses a precise `edit` (find-and-replace) tool rather than rewriting entire files, significantly reducing token consumption and preventing hallucinations.
* 💾 **Session Persistence & Resume**: Automatically records your interaction history to structured JSONL files. Resume any previous task with `--continue`.
* 🛡️ **Interactive Safety & Signal Handling**: Gracefully handles interrupts (`Ctrl+C`) and offers manual control before executing long-running tasks.

---

## 🚀 Quick Start (Get Started in 1 Minute)

### 1. Build AXON

Ensure you have [Go](https://go.dev/) (1.21+) installed, then run:

```bash
git clone https://github.com/zane0x/AXON.git
cd AXON
go build -o axon main.go
```

### 2. Configure Environment

Set up your API endpoints. AXON supports any OpenAI-compatible base URL.

```bash
# Example: Using OpenAI API
export LLM_END_PROT="https://api.openai.com/v1"
export LLM_TOKEN="sk-proj-..."
export LLM_MODEL="gpt-4o"  # Optional, defaults to gemini-3-flash-agent

# Example: Using DeepSeek API
export LLM_END_PROT="https://api.deepseek.com/v1"
export LLM_TOKEN="sk-..."
export LLM_MODEL="deepseek-coder"
```

### 3. Run AXON

Start a new interactive session in your current working directory:

```bash
./axon
```

---

## 🛠️ Built-in Capabilities (Tools)

AXON exposes a set of rich, structured primitives that let the AI inspect and modify files safely:

| Tool | Action | Description |
| :--- | :--- | :--- |
| `read` | **Read File** | Reads the full text content of a file (safe/read-only). |
| `write` | **Write File** | Writes a brand new file or completely replaces the contents of an existing one. |
| `edit` | **Surgical Replace** | Replaces the first verbatim match of `old_text` with `new_text`. Safe and token-efficient. |
| `bash` | **Subprocess Bash** | Runs terminal commands (e.g. `go test`, `npm run build`, `git status`). |

---

## 📖 Usage & CLI Options

AXON provides advanced CLI flags to manage your sessions:

```text
Usage: axon [options]

Options:
  --continue, -c [path]   resume the most recent session (or a specific JSONL file)
  --model, -m <model>     specify the model to use (defaults to LLM_MODEL env var or "gemini-3-flash-agent")
  --list, -l              list all saved sessions
  --help, -h              show this help

Environment variables:
  LLM_TOKEN      API key for the LLM provider
  LLM_END_PROT   Base URL of the LLM API endpoint
  LLM_MODEL      Default model name to use
```

### Resume a Session
To resume your last session and continue where you left off:
```bash
./axon --continue
```
Or resume a specific session file:
```bash
./axon --continue ~/.axon/sessions/2026-08-24T22-00-00.jsonl
```

---

## 🏗️ Architecture

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

### Core Design Principles

1. **Runtime over Prompt**: Reliable agents require explicit state machine and lifecycle hooks inside the engine, not just complex prompt engineering.
2. **Model Agnostic**: Keep the execution loop strictly decoupled from LLM providers.
3. **Tool First**: Treat capabilities as first-class, typed Go structures instead of simple JSON fragments.
4. **Observable**: Every tool call, event, and state transition emits structured trace information.
5. **Interruptible**: Allow developer intervention, pauses, and restarts at any phase of the execution loop.

---

## 🗺️ Roadmap & Status

> 🚧 **Early Development**: AXON is actively evolving. APIs and file structures are subject to change.

### Completed / In Progress
- [x] Basic REPL & agent execution loops.
- [x] Session persistence to JSONL.
- [x] Multi-turn session resumption (`--continue`).
- [x] Precise `edit` find-and-replace tool.
- [x] Dynamic model switching (`--model` / `LLM_MODEL`).

### Upcoming Milestones
- [ ] **Model Context Protocol (MCP)** support for custom tool integration.
- [ ] Parallel tool execution & sub-agent orchestration.
- [ ] Streaming response support.
- [ ] Terminal UI (TUI) for interactive task tracing.

---

## 🤝 Contributing

Contributions are highly welcome! Whether it is fixing a bug, suggesting a feature, or writing a custom tool:

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/amazing-feature`).
3. Commit your changes (`git commit -m 'feat: add amazing capability'`).
4. Push to the branch (`git push origin feature/amazing-feature`).
5. Open a Pull Request.

---

## 📄 License

AXON is open-source software licensed under the MIT License. (See [LICENSE](LICENSE) for details).
