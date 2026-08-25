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
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
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
		preview := session.FirstUserMessage()
		if len([]rune(preview)) > 60 {
			preview = string([]rune(preview)[:60]) + "…"
		}
		engine.PrintSessionResumed(session.SessionID(), users+assistants, preview)
	} else {
		engine.PrintSessionNew(session.SessionID())
	}
	engine.PrintSessionFile(session.SessionPath())

	// ── REPL ───────────────────────────────────────────────────────────────────
	homeDir, err := os.UserHomeDir()
	var historyFile string
	if err == nil {
		historyDir := filepath.Join(homeDir, ".axon")
		_ = os.MkdirAll(historyDir, 0755)
		historyFile = filepath.Join(historyDir, "history")
	}

	completer := readline.NewPrefixCompleter(
		readline.PcItem("/exit"),
		readline.PcItem("/quit"),
		readline.PcItem("/clear"),
		readline.PcItem("/help"),
	)

	l, err := readline.NewEx(&readline.Config{
		Prompt:          "\x1b[1;35m> \x1b[0m", // Bold Pink prompt
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize readline: %v\n", err)
		os.Exit(1)
	}
	defer l.Close()

	fmt.Println("\x1b[1;32mInteractive CLI Mode.\x1b[0m Type \x1b[1;36m/help\x1b[0m for assistance.")

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			// Ctrl-C: cancel/clear the current line
			continue
		} else if err == io.EOF {
			fmt.Println("Bye~Bye~")
			return
		}

		prompt := strings.TrimSpace(line)
		if prompt == "" {
			continue
		}

		// Built-in Commands
		if prompt == "/exit" || prompt == "/quit" || prompt == "exit" || prompt == "quit" {
			fmt.Println("Bye~Bye~")
			return
		}
		if prompt == "/clear" {
			fmt.Print("\x1b[H\x1b[2J") // Clear screen
			continue
		}
		if prompt == "/help" {
			printInteractiveHelp()
			continue
		}

		// Multi-line Input Handling (triple-quotes block)
		if strings.HasPrefix(prompt, `"""`) {
			var builder strings.Builder
			initial := strings.TrimPrefix(prompt, `"""`)
			if initial != "" {
				builder.WriteString(initial)
				builder.WriteByte('\n')
			}
			l.SetPrompt("\x1b[2;90m... \x1b[0m") // Dim gray continuation prompt
			for {
				subLine, subErr := l.Readline()
				if subErr != nil {
					break
				}
				subTrimmed := strings.TrimSpace(subLine)
				if subTrimmed == `"""` {
					break
				}
				builder.WriteString(subLine)
				builder.WriteByte('\n')
			}
			l.SetPrompt("\x1b[1;35m> \x1b[0m") // Restore original prompt
			prompt = strings.TrimSpace(builder.String())
			if prompt == "" {
				continue
			}
		} else if strings.HasSuffix(prompt, "\\") {
			// Multi-line Input Handling (backslash continuation)
			var builder strings.Builder
			builder.WriteString(strings.TrimSuffix(prompt, "\\"))
			builder.WriteByte('\n')
			l.SetPrompt("\x1b[2;90m... \x1b[0m") // Dim gray continuation prompt
			for {
				subLine, subErr := l.Readline()
				if subErr != nil {
					break
				}
				trimmed := strings.TrimSpace(subLine)
				if strings.HasSuffix(trimmed, "\\") {
					builder.WriteString(strings.TrimSuffix(subLine, "\\"))
					builder.WriteByte('\n')
				} else {
					builder.WriteString(subLine)
					break
				}
			}
			l.SetPrompt("\x1b[1;35m> \x1b[0m") // Restore original prompt
			prompt = strings.TrimSpace(builder.String())
			if prompt == "" {
				continue
			}
		}

		err = engine.AgentLoop(ctx, &client, model, prompt, toolContainer, systemPrompt, session)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				engine.PrintInterrupted()
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "\x1b[1;31magent error: %v\x1b[0m\n", err)
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

	// --continue 但未指定路径：寻找最近的 sessions
	summaries, err := engine.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("cannot list sessions: %w", err)
	}

	if len(summaries) == 0 {
		fmt.Println("[session] no existing session found, starting new session")
		return engine.NewSessionManager()
	}

	// 如果只有 1 个 session，自动加载它
	if len(summaries) == 1 {
		fmt.Printf("[session] automatically resuming the only session: %s\n", summaries[0].ID)
		return engine.LoadSessionManager(summaries[0].Path)
	}

	// 如果有多个 session，显示交互式菜单让用户选择，限制最大展示数量（例如 7 个）
	maxShow := 7
	if len(summaries) < maxShow {
		maxShow = len(summaries)
	}

	fmt.Println("\x1b[1;36mSelect a session to resume:\x1b[0m")
	for i := 0; i < maxShow; i++ {
		s := summaries[i]
		modStr := s.ModTime.Format("2006-01-02 15:04")
		preview := s.FirstUserMsg
		if preview == "" {
			preview = "(empty)"
		}
		// 列出选项
		fmt.Printf("  \x1b[1;33m[%d]\x1b[0m \x1b[90m%s\x1b[0m (%d msgs) %s\n", i+1, modStr, s.EntryCount, preview)
	}
	fmt.Printf("  \x1b[1;33m[%d]\x1b[0m Start a brand new session\n", maxShow+1)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter selection [1-%d, default 1]: ", maxShow+1)
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			// 默认选 1 (最近的会话)
			return engine.LoadSessionManager(summaries[0].Path)
		}

		var choice int
		_, scanErr := fmt.Sscanf(text, "%d", &choice)
		if scanErr != nil || choice < 1 || choice > maxShow+1 {
			fmt.Printf("Invalid selection. Please enter a number between 1 and %d.\n", maxShow+1)
			continue
		}

		if choice == maxShow+1 {
			fmt.Println("[session] starting new session")
			return engine.NewSessionManager()
		}

		selected := summaries[choice-1]
		return engine.LoadSessionManager(selected.Path)
	}
}

func printInteractiveHelp() {
	fmt.Print(`
Interactive CLI Commands:
  /exit, /quit      Exit the program
  /clear            Clear screen
  /help             Show this help information

Multi-line input features:
  - Start input with """ to begin a multi-line block, and type """ on its own line to submit.
  - Or end any line with a backslash \ to continue typing on the next line.

`)
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
