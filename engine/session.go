package engine

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openai/openai-go"
)

// EntryType 枚举 session 条目的类型，对应 JSONL 中 "type" 字段。
// pi 使用字符串枚举区分不同角色的消息，方便版本迁移和过滤。
type EntryType string

const (
	EntryTypeUser      EntryType = "user"
	EntryTypeAssistant EntryType = "assistant"
	// tool_result 附属于某次 assistant 的 tool_call，需要特殊重建逻辑
	EntryTypeToolResult EntryType = "tool_result"
	EntryTypeSummary    EntryType = "summary" // 预留给第 5 课上下文压缩
)

// SessionEntry 是写入 JSONL 的最小单元。
// 每行一个 JSON 对象，字段保持向前兼容（新增字段不破坏旧数据）。
type SessionEntry struct {
	Type      EntryType `json:"type"`
	ID        string    `json:"id"`
	Timestamp int64     `json:"timestamp"`
	// Content 存储消息的文本内容；对于 assistant tool_call 消息，
	// 文本部分存这里，tool_calls 存 ToolCalls。
	Content string `json:"content,omitempty"`
	// ToolCalls 仅 assistant 类型且有工具调用时写入。
	ToolCalls []SessionToolCall `json:"tool_calls,omitempty"`
	// ToolCallID 仅 tool_result 类型使用，关联到 assistant 消息中对应的 tool_call。
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName 仅 tool_result 类型使用，便于日志展示。
	ToolName string `json:"tool_name,omitempty"`
}

// SessionToolCall 对应 openai tool_call 的最小持久化形式。
type SessionToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SessionManager 负责一个会话文件的读写生命周期。
// 会话文件格式：JSONL（每行一个 SessionEntry JSON）。
// 默认路径：~/.axon/sessions/<id>.jsonl
type SessionManager struct {
	// sessionID 是本次会话的唯一标识，也是文件名（不含扩展名）。
	sessionID string
	// sessionPath 是 JSONL 文件的完整路径。
	sessionPath string
	// sessionList 是内存中已加载的全部条目（含本次新增）。
	sessionList []SessionEntry
}

// sessionsDir 返回默认 sessions 目录（~/.axon/sessions）。
func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home dir: %w", err)
	}
	return filepath.Join(home, ".axon", "sessions"), nil
}

// NewSessionManager 创建一个全新会话（生成新 ID，不读磁盘）。
func NewSessionManager() (*SessionManager, error) {
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}
	return NewSessionManagerInDir(dir)
}

// NewSessionManagerInDir 在指定目录创建一个全新会话。
// 除测试外，也可用于调用方显式管理 session 存储位置。
func NewSessionManagerInDir(dir string) (*SessionManager, error) {
	if dir == "" {
		return nil, fmt.Errorf("session directory must not be empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create sessions dir: %w", err)
	}
	id := NewID()
	return &SessionManager{
		sessionID:   id,
		sessionPath: filepath.Join(dir, id+".jsonl"),
	}, nil
}

// LoadSessionManager 从指定 JSONL 文件加载已有会话（用于 --continue）。
func LoadSessionManager(path string) (*SessionManager, error) {
	entries, err := parseSessionFile(path)
	if err != nil {
		return nil, err
	}
	// 从文件名提取 ID（去掉目录和 .jsonl 后缀）
	base := filepath.Base(path)
	id := strings.TrimSuffix(base, ".jsonl")
	return &SessionManager{
		sessionID:   id,
		sessionPath: path,
		sessionList: entries,
	}, nil
}

// FindMostRecentSession 返回 sessions 目录中修改时间最新的 JSONL 文件路径。
// 若目录为空或不存在，返回 ("", nil)。
func FindMostRecentSession() (string, error) {
	dir, err := sessionsDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read sessions dir: %w", err)
	}

	// 过滤 .jsonl 文件并按修改时间降序排列
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
		})
	}
	if len(files) == 0 {
		return "", nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	return files[0].path, nil
}

// SessionID 返回当前会话 ID。
func (s *SessionManager) SessionID() string {
	return s.sessionID
}

// SessionPath 返回当前会话文件路径。
func (s *SessionManager) SessionPath() string {
	return s.sessionPath
}

// AddUserEntry 追加一条用户消息并持久化。
func (s *SessionManager) AddUserEntry(content string) error {
	return s.addEntry(EntryTypeUser, content, "", "", nil)
}

// AddAssistantEntry 追加一条 assistant 消息（含可选 tool_calls）并持久化。
func (s *SessionManager) AddAssistantEntry(content string, toolCalls []SessionToolCall) error {
	return s.addEntry(EntryTypeAssistant, content, "", "", toolCalls)
}

// AddToolResultEntry 追加一条工具执行结果并持久化。
func (s *SessionManager) AddToolResultEntry(toolCallID, toolName, content string) error {
	return s.addEntry(EntryTypeToolResult, content, toolCallID, toolName, nil)
}

// addEntry 是内部统一写入方法：追加到内存列表并写一行 JSONL 到磁盘。
// pi 采用逐行追加写入（而非整体重写），保证崩溃时只丢失最后一行，其余条目安全。
func (s *SessionManager) addEntry(
	entryType EntryType,
	content string,
	toolCallID string,
	toolName string,
	toolCalls []SessionToolCall,
) error {
	entry := SessionEntry{
		Type:       entryType,
		ID:         NewID(),
		Timestamp:  time.Now().Unix(),
		Content:    content,
		ToolCalls:  toolCalls,
		ToolCallID: toolCallID,
		ToolName:   toolName,
	}
	s.sessionList = append(s.sessionList, entry)
	return s.appendEntryToFile(entry)
}

// appendEntryToFile 将单个条目序列化为 JSON 并追加一行到 JSONL 文件。
// 以 O_APPEND 打开，保证并发安全（单进程场景下足够）。
func (s *SessionManager) appendEntryToFile(entry SessionEntry) error {
	f, err := os.OpenFile(s.sessionPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open session file: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cannot marshal session entry: %w", err)
	}
	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

// BuildMessages 将内存中的 sessionList 重建为 openai 消息列表。
// 重建规则（对应 pi 的 buildSessionContext）：
//   - user      → openai.UserMessage
//   - assistant → openai.AssistantMessage（含 tool_calls 时用 ToParam 还原）
//   - tool_result → openai.ToolMessage
//   - summary   → openai.SystemMessage（压缩摘要，第 5 课启用）
func (s *SessionManager) BuildMessages() []openai.ChatCompletionMessageParamUnion {
	var msgs []openai.ChatCompletionMessageParamUnion
	for _, e := range s.sessionList {
		switch e.Type {
		case EntryTypeUser:
			msgs = append(msgs, openai.UserMessage(e.Content))

		case EntryTypeAssistant:
			if len(e.ToolCalls) == 0 {
				msgs = append(msgs, openai.AssistantMessage(e.Content))
			} else {
				// 有 tool_call 时需要构造带 tool_calls 字段的 assistant 消息
				msgs = append(msgs, buildAssistantMessageWithToolCalls(e))
			}

		case EntryTypeToolResult:
			msgs = append(msgs, openai.ToolMessage(e.Content, e.ToolCallID))

		case EntryTypeSummary:
			// 压缩摘要作为新的 system 消息插入，替代被压缩掉的旧历史
			msgs = append(msgs, openai.SystemMessage(e.Content))
		}
	}
	return msgs
}

// buildAssistantMessageWithToolCalls 将持久化的 SessionEntry 还原为
// 带 tool_calls 的 assistant ChatCompletionMessageParam。
func buildAssistantMessageWithToolCalls(e SessionEntry) openai.ChatCompletionMessageParamUnion {
	tcs := make([]openai.ChatCompletionMessageToolCallParam, 0, len(e.ToolCalls))
	for _, tc := range e.ToolCalls {
		tcs = append(tcs, openai.ChatCompletionMessageToolCallParam{
			ID:   tc.ID,
			Type: "function",
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}
	msg := openai.ChatCompletionAssistantMessageParam{
		Role:      "assistant",
		ToolCalls: tcs,
	}
	if e.Content != "" {
		msg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(e.Content),
		}
	}
	return openai.ChatCompletionMessageParamUnion{OfAssistant: &msg}
}

// parseSessionFile 从磁盘读取 JSONL 文件，逐行解析为 SessionEntry 列表。
// 空行和解析失败的行会被跳过（容错设计，兼容截断写入）。
func parseSessionFile(path string) ([]SessionEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open session file %s: %w", path, err)
	}
	defer f.Close()

	var entries []SessionEntry
	scanner := bufio.NewScanner(f)
	// 单行最大 10 MB，防止超大工具输出卡住 Scanner
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry SessionEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// 跳过损坏行，不中断加载（与 pi 的容错策略一致）
			fmt.Fprintf(os.Stderr, "[session] skip malformed line %d: %v\n", lineNum, err)
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file: %w", err)
	}
	return entries, nil
}

// ─── 查询辅助 ─────────────────────────────────────────────────────────────────

// Len 返回内存中已加载的条目总数。
func (s *SessionManager) Len() int {
	return len(s.sessionList)
}

// Entries 返回内存中所有条目的只读切片副本，供外部遍历（不可修改内部状态）。
func (s *SessionManager) Entries() []SessionEntry {
	cp := make([]SessionEntry, len(s.sessionList))
	copy(cp, s.sessionList)
	return cp
}

// FirstUserMessage 返回 session 中第一条 user 消息的文本内容。
// 若不存在则返回空字符串。常用于 list 展示时生成摘要标题。
func (s *SessionManager) FirstUserMessage() string {
	for _, e := range s.sessionList {
		if e.Type == EntryTypeUser {
			return e.Content
		}
	}
	return ""
}

// MessageCount 返回各类型条目的数量统计。
func (s *SessionManager) MessageCount() (users, assistants, toolResults int) {
	for _, e := range s.sessionList {
		switch e.Type {
		case EntryTypeUser:
			users++
		case EntryTypeAssistant:
			assistants++
		case EntryTypeToolResult:
			toolResults++
		}
	}
	return
}

// ─── 会话列表 ─────────────────────────────────────────────────────────────────

// SessionSummary 是 ListSessions 返回的单条会话摘要，用于 --list 展示。
type SessionSummary struct {
	// ID 是会话唯一标识（文件名去掉 .jsonl）。
	ID string
	// Path 是 JSONL 文件的完整路径。
	Path string
	// ModTime 是文件最后修改时间（即最后一次写入时间）。
	ModTime time.Time
	// EntryCount 是条目总数（含 tool_result）。
	EntryCount int
	// FirstUserMsg 是第一条 user 消息的文本内容（截断到 80 字符），用于标题预览。
	FirstUserMsg string
}

// ListSessions 扫描 sessions 目录，返回所有会话的摘要列表，按修改时间降序排列。
// 若目录不存在或为空，返回 (nil, nil)。
// 为了构造摘要，每个文件都会被完整解析——适合会话数量不多（几百个以内）的场景。
func ListSessions() ([]SessionSummary, error) {
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read sessions dir: %w", err)
	}

	var summaries []SessionSummary
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".jsonl") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, de.Name())
		id := strings.TrimSuffix(de.Name(), ".jsonl")

		// 轻量加载：解析 JSONL 只为获取第一条 user 消息和条目数
		entries, err := parseSessionFile(path)
		if err != nil {
			// 损坏文件只记录 ID，不跳过（让用户看到它存在）
			summaries = append(summaries, SessionSummary{
				ID:      id,
				Path:    path,
				ModTime: info.ModTime(),
			})
			continue
		}

		firstUser := ""
		for _, e := range entries {
			if e.Type == EntryTypeUser {
				firstUser = e.Content
				break
			}
		}
		// 截断过长的预览文本
		const previewLen = 80
		if len([]rune(firstUser)) > previewLen {
			firstUser = string([]rune(firstUser)[:previewLen]) + "…"
		}

		summaries = append(summaries, SessionSummary{
			ID:           id,
			Path:         path,
			ModTime:      info.ModTime(),
			EntryCount:   len(entries),
			FirstUserMsg: firstUser,
		})
	}

	// 按修改时间降序排列（最新的在前）
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ModTime.After(summaries[j].ModTime)
	})
	return summaries, nil
}

// ─── 会话变更 ─────────────────────────────────────────────────────────────────

// AddSummaryEntry 追加一条上下文压缩摘要并持久化（第 5 课使用）。
// 写入后，BuildMessages 会将其还原为 SystemMessage 插入到消息列表头部。
func (s *SessionManager) AddSummaryEntry(content string) error {
	return s.addEntry(EntryTypeSummary, content, "", "", nil)
}

// TruncateAfter 将内存中的 sessionList 截断到指定条目数，并重写整个 JSONL 文件。
// 用于上下文压缩后去掉被摘要替代的旧条目。
// 注意：这是破坏性操作，调用前应确认截断点正确。
func (s *SessionManager) TruncateAfter(keepCount int) error {
	if keepCount < 0 {
		keepCount = 0
	}
	if keepCount >= len(s.sessionList) {
		return nil // 无需截断
	}
	s.sessionList = s.sessionList[:keepCount]
	return s.rewriteFile()
}

// rewriteFile 将内存中当前的 sessionList 整体重写到 JSONL 文件。
// 仅在需要修改历史记录时（如 TruncateAfter）才调用；正常追加用 appendEntryToFile。
func (s *SessionManager) rewriteFile() error {
	f, err := os.OpenFile(s.sessionPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open session file for rewrite: %w", err)
	}
	defer f.Close()

	for _, entry := range s.sessionList {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("cannot marshal entry %s: %w", entry.ID, err)
		}
		if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
			return fmt.Errorf("cannot write entry %s: %w", entry.ID, err)
		}
	}
	return nil
}

// ─── ID 生成 ──────────────────────────────────────────────────────────────────

// NewID 生成 URL-safe base64 编码的 16 字节随机 ID（22 字符，无填充）。
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
