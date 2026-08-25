package engine

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/glamour"
)

// ── UI 输出（给用户看，写到 stdout）──────────────────────────────────────────

var (
	markdownRenderer *glamour.TermRenderer
	rendererOnce     sync.Once
)

func renderMarkdown(content string) string {
	rendererOnce.Do(func() {
		// WithAutoStyle 自动适应终端颜色主题
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(120),
		)
		if err == nil {
			markdownRenderer = r
		}
	})

	if markdownRenderer == nil {
		return content
	}
	out, err := markdownRenderer.Render(content)
	if err != nil {
		return content
	}
	return out
}

// PrintAssistant 打印 assistant 的最终回复。
func PrintAssistant(content string) {
	if content != "" {
		rendered := renderMarkdown(content)
		fmt.Printf("\x1b[1;34m🤖 Assistant:\x1b[0m\n%s\n", rendered)
	}
}

// PrintThinking 打印 thinking 状态及中间回复。
func PrintThinking(content string) {
	if content != "" {
		fmt.Printf("\x1b[2;90m💭 Thoughts:\x1b[0m\n\x1b[3;90m%s\x1b[0m\n\n", content)
	}
}

// PrintToolCall 打印 agent 正在调用的工具名和参数。
func PrintToolCall(name, arguments string) {
	fmt.Printf("\x1b[1;36m🔧 Calling tool [%s]\x1b[0m \x1b[36m%s\x1b[0m\n", name, arguments)
}

// PrintToolResult 打印工具执行结果。
func PrintToolResult(displayResult string) {
	fmt.Printf("\x1b[1;32m📝 Tool Result:\x1b[0m\n%s\n\n", displayResult)
}

// PrintSessionResumed 打印续聊会话的基本信息。
func PrintSessionResumed(sessionID string, turns int, firstMsg string) {
	fmt.Printf("\x1b[90m[session] resumed %s (%d turns)\x1b[0m\n", sessionID, turns)
	if firstMsg != "" {
		fmt.Printf("\x1b[90m[session] started with: %q\x1b[0m\n", firstMsg)
	}
}

// PrintSessionNew 打印新会话的基本信息。
func PrintSessionNew(sessionID string) {
	fmt.Printf("\x1b[90m[session] new session %s\x1b[0m\n", sessionID)
}

// PrintSessionFile 打印会话文件路径。
func PrintSessionFile(path string) {
	fmt.Printf("\x1b[90m[session] file: %s\x1b[0m\n\n", path)
}

// PrintInterrupted 打印用户中断提示。
func PrintInterrupted() {
	fmt.Print("\n\x1b[1;31minterrupted\x1b[0m")
}

// ── 内部警告（非致命，写到 stderr）──────────────────────────────────────────

// WarnSessionPersist 打印 session 持久化失败的警告。
func WarnSessionPersist(context string, err error) {
	fmt.Fprintf(os.Stderr, "[session] warn: failed to persist %s: %v\n", context, err)
}

// WarnResponseTruncated 打印响应被截断的警告。
func WarnResponseTruncated() {
	fmt.Fprintln(os.Stderr, "[warning] response truncated with pending tool calls")
}

// WarnSessionSkipLine 打印跳过损坏 JSONL 行的警告。
func WarnSessionSkipLine(lineNum int, err error) {
	fmt.Fprintf(os.Stderr, "[session] skip malformed line %d: %v\n", lineNum, err)
}
