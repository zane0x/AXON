package engine

import (
	"fmt"
	"os"
)

// ── UI 输出（给用户看，写到 stdout）──────────────────────────────────────────

// PrintAssistant 打印 assistant 的最终回复。
func PrintAssistant(content string) {
	fmt.Printf("assistant: %s\n", content)
}

// PrintToolCall 打印 agent 正在调用的工具名和参数。
func PrintToolCall(name, arguments string) {
	fmt.Printf("tool call: %s(%s)\n", name, arguments)
}

// PrintSessionResumed 打印续聊会话的基本信息。
func PrintSessionResumed(sessionID string, turns int, firstMsg string) {
	fmt.Printf("[session] resumed %s (%d turns)\n", sessionID, turns)
	if firstMsg != "" {
		fmt.Printf("[session] started with: %q\n", firstMsg)
	}
}

// PrintSessionNew 打印新会话的基本信息。
func PrintSessionNew(sessionID string) {
	fmt.Printf("[session] new session %s\n", sessionID)
}

// PrintSessionFile 打印会话文件路径。
func PrintSessionFile(path string) {
	fmt.Printf("[session] file: %s\n\n", path)
}

// PrintInterrupted 打印用户中断提示。
func PrintInterrupted() {
	fmt.Print("\ninterrupted")
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
