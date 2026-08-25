package main

import (
	"axon/engine"
	"axon/tools"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// ── 命令行参数解析 ─────────────────────────────────────────────────────────
	// 支持以下模式：
	//   axon                     → 全新会话
	//   axon --continue / -c     → 续接最近会话
	//   axon --continue <path>   → 续接指定 JSONL 文件
	//   axon --list              → 列出所有会话摘要
	args := os.Args[1:]
	var (
		continueMode bool   // 是否续聊
		continuePath string // 指定续聊的文件路径（可选）
		listMode     bool   // 是否只列出会话
	)
	modelName := os.Getenv("LLM_MODEL")
	if modelName == "" {
		modelName = "gemini-3-flash-agent"
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--continue", "-c":
			continueMode = true
			// 若下一个参数不是 flag（不以 - 开头），则视为路径
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				continuePath = args[i]
			}
		case "--model", "-m":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				modelName = args[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --model or -m requires an argument")
				os.Exit(1)
			}
		case "--list", "-l":
			listMode = true
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			printUsage()
			os.Exit(1)
		}
	}

	// ── --list 模式 ────────────────────────────────────────────────────────────
	if listMode {
		if err := runListMode(); err != nil {
			fmt.Fprintf(os.Stderr, "list error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// ── resolve cwd ───────────────────────────────────────────────────────────
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
		os.Exit(1)
	}

	//初始化model
	model := engine.Model{
		ID:            modelName,
		ContextWindow: 128000,
		MaxTokens:     12800,
	}

	// ── 工具注册 ───────────────────────────────────────────────────────────────
	toolContainer := tools.NewToolContainer()
	toolContainer.RegisterTool(&tools.BashTool{})
	toolContainer.RegisterTool(&tools.ReadTool{})
	toolContainer.RegisterTool(&tools.WriteTool{})
	toolContainer.RegisterTool(&tools.EditTool{})

	// ── LLM 客户端 ─────────────────────────────────────────────────────────────
	apiKey := os.Getenv("LLM_TOKEN")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "LLM_TOKEN env variable not set, exit")
		os.Exit(1)
	}
	baseURL := os.Getenv("LLM_END_PROT")
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "LLM_END_PROT env variable not set, exit")
		os.Exit(1)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	systemPrompt := engine.BuildSystemPrompt(engine.SystemPromptOptions{
		Cwd:       cwd,
		Container: toolContainer,
	})

	// ── Session 初始化 ─────────────────────────────────────────────────────────
	session, err := initSession(continueMode, continuePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session error: %v\n", err)
		os.Exit(1)
	}

	if continueMode && session.Len() > 0 {
		users, assistants, _ := session.MessageCount()
		fmt.Printf("[session] resumed %s (%d turns)\n", session.SessionID(), users+assistants)
		preview := session.FirstUserMessage()
		if len([]rune(preview)) > 60 {
			preview = string([]rune(preview)[:60]) + "…"
		}
		if preview != "" {
			fmt.Printf("[session] started with: %q\n", preview)
		}
	} else {
		fmt.Printf("[session] new session %s\n", session.SessionID())
	}
	fmt.Printf("[session] file: %s\n\n", session.SessionPath())

	// ── REPL ───────────────────────────────────────────────────────────────────
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, err := reader.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			os.Exit(1)
		}

		prompt := strings.TrimSpace(text)
		if prompt == "" {
			continue
		}
		if prompt == "exit" || prompt == "quit" {
			fmt.Println("Bye~Bye~")
			return
		}

		err = engine.AgentLoop(ctx, &client, model, prompt, toolContainer, systemPrompt, session)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Print("\ninterrupted")
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "agent error: %v\n", err)
			os.Exit(1)
		}
	}
}

// initSession 根据命令行参数决定创建新 session 还是加载已有 session。
func initSession(continueMode bool, continuePath string) (*engine.SessionManager, error) {
	if !continueMode {
		return engine.NewSessionManager()
	}

	// --continue 且指定了路径
	if continuePath != "" {
		return engine.LoadSessionManager(continuePath)
	}

	// --continue 但未指定路径：自动找最近的 session
	recent, err := engine.FindMostRecentSession()
	if err != nil {
		return nil, fmt.Errorf("cannot find recent session: %w", err)
	}
	if recent == "" {
		fmt.Println("[session] no existing session found, starting new session")
		return engine.NewSessionManager()
	}
	return engine.LoadSessionManager(recent)
}

// runListMode 列出所有会话摘要并打印到 stdout。
func runListMode() error {
	summaries, err := engine.ListSessions()
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		fmt.Println("no sessions found")
		return nil
	}

	fmt.Printf("%-22s  %-20s  %5s  %s\n", "ID", "MODIFIED", "MSGS", "FIRST MESSAGE")
	fmt.Println(strings.Repeat("-", 100))
	for _, s := range summaries {
		modStr := s.ModTime.Format(time.RFC3339)[:16] // "2006-01-02T15:04"
		preview := s.FirstUserMsg
		if preview == "" {
			preview = "(empty)"
		}
		fmt.Printf("%-22s  %-20s  %5d  %s\n", s.ID, modStr, s.EntryCount, preview)
	}
	return nil
}

// printUsage 输出帮助信息。
func printUsage() {
	fmt.Println(`Usage: axon [options]

Options:
  --continue, -c [path]   resume the most recent session (or a specific JSONL file)
  --model, -m <model>     specify the model to use (defaults to LLM_MODEL env var or "gemini-3-flash-agent")
  --list, -l              list all saved sessions
  --help, -h              show this help

Environment variables:
  LLM_TOKEN      API key for the LLM provider
  LLM_END_PROT   Base URL of the LLM API endpoint
  LLM_MODEL      Default model name to use`)
}
