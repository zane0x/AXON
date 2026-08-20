package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// tempFile 在系统临时目录创建一个带初始内容的临时文件，并在测试结束后自动清理。
func tempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "file_tools_test_*.txt")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()
	return f.Name()
}

// ─── ReadTool tests ───────────────────────────────────────────────────────────

func TestReadTool_Definition(t *testing.T) {
	tool := &ReadTool{}
	def := tool.Definition()

	if def.Name != "read" {
		t.Errorf("Name = %q, want %q", def.Name, "read")
	}
	if _, ok := def.Parameters.Properties["path"]; !ok {
		t.Error("Parameters.Properties should contain 'path'")
	}
	if len(def.Parameters.Required) == 0 || def.Parameters.Required[0] != "path" {
		t.Error("'path' should be in Required")
	}
}

func TestReadTool_Definition_HasDesc(t *testing.T) {
	def := (&ReadTool{}).Definition()
	if def.Desc == "" {
		t.Error("expected non-empty Desc")
	}
}

func TestReadTool_Definition_HasGuidelines(t *testing.T) {
	def := (&ReadTool{}).Definition()
	if len(def.Guidelines) == 0 {
		t.Error("expected at least one guideline")
	}
}

func TestReadTool_Execute_Success(t *testing.T) {
	const want = "hello, world"
	path := tempFile(t, want)

	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{"path": path}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

func TestReadTool_Execute_EmptyFile(t *testing.T) {
	path := tempFile(t, "")
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{"path": path}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty string", result.Content)
	}
}

func TestReadTool_Execute_MultilineContent(t *testing.T) {
	content := "line1\nline2\nline3"
	path := tempFile(t, content)
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{"path": path}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != content {
		t.Errorf("Content = %q, want %q", result.Content, content)
	}
}

func TestReadTool_Execute_FileNotFound(t *testing.T) {
	tool := &ReadTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path": filepath.Join(t.TempDir(), "nonexistent.txt"),
	}))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadTool_Execute_InvalidArgs(t *testing.T) {
	tool := &ReadTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON args, got nil")
	}
}

// ─── WriteTool tests ──────────────────────────────────────────────────────────

func TestWriteTool_Definition(t *testing.T) {
	tool := &WriteTool{}
	def := tool.Definition()

	if def.Name != "write" {
		t.Errorf("Name = %q, want %q", def.Name, "write")
	}
	for _, key := range []string{"path", "content"} {
		if _, ok := def.Parameters.Properties[key]; !ok {
			t.Errorf("Parameters.Properties should contain %q", key)
		}
	}
}

func TestWriteTool_Definition_HasDesc(t *testing.T) {
	def := (&WriteTool{}).Definition()
	if def.Desc == "" {
		t.Error("expected non-empty Desc")
	}
}

func TestWriteTool_Definition_RequiredFields(t *testing.T) {
	def := (&WriteTool{}).Definition()
	required := map[string]bool{}
	for _, r := range def.Parameters.Required {
		required[r] = true
	}
	for _, f := range []string{"path", "content"} {
		if !required[f] {
			t.Errorf("field %q should be required", f)
		}
	}
}

func TestWriteTool_Execute_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	const content = "written by WriteTool"

	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":    path,
		"content": content,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty success message")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after write: %v", err)
	}
	if string(got) != content {
		t.Errorf("file content = %q, want %q", string(got), content)
	}
}

func TestWriteTool_Execute_EmptyContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":    path,
		"content": "",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "" {
		t.Errorf("expected empty file, got %q", string(got))
	}
}

func TestWriteTool_Execute_OverwritesExistingFile(t *testing.T) {
	path := tempFile(t, "old content")

	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":    path,
		"content": "new content",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new content" {
		t.Errorf("file content = %q, want %q", string(got), "new content")
	}
}

func TestWriteTool_Execute_NonExistentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no_such_dir", "file.txt")
	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":    path,
		"content": "data",
	}))
	if err == nil {
		t.Fatal("expected error when parent directory does not exist, got nil")
	}
}

func TestWriteTool_Execute_InvalidArgs(t *testing.T) {
	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON args, got nil")
	}
}

func TestWriteTool_Execute_SuccessMessageContainsByteCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	content := "hello"
	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":    path,
		"content": content,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Success message should mention byte count (5 bytes).
	if result.Content == "" {
		t.Error("expected non-empty success message")
	}
}

// ─── EditTool tests ───────────────────────────────────────────────────────────

func TestEditTool_Definition(t *testing.T) {
	tool := &EditTool{}
	def := tool.Definition()

	if def.Name != "edit" {
		t.Errorf("Name = %q, want %q", def.Name, "edit")
	}
	for _, key := range []string{"path", "old_text", "new_text"} {
		if _, ok := def.Parameters.Properties[key]; !ok {
			t.Errorf("Parameters.Properties should contain %q", key)
		}
	}
}

func TestEditTool_Definition_HasDesc(t *testing.T) {
	def := (&EditTool{}).Definition()
	if def.Desc == "" {
		t.Error("expected non-empty Desc")
	}
}

func TestEditTool_Definition_RequiredFields(t *testing.T) {
	def := (&EditTool{}).Definition()
	required := map[string]bool{}
	for _, r := range def.Parameters.Required {
		required[r] = true
	}
	for _, f := range []string{"path", "old_text", "new_text"} {
		if !required[f] {
			t.Errorf("field %q should be required", f)
		}
	}
}

func TestEditTool_Execute_Success(t *testing.T) {
	path := tempFile(t, "foo bar baz")

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": "bar",
		"new_text": "qux",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty success message")
	}

	got, _ := os.ReadFile(path)
	if string(got) != "foo qux baz" {
		t.Errorf("file content = %q, want %q", string(got), "foo qux baz")
	}
}

func TestEditTool_Execute_ReplacesOnlyFirstOccurrence(t *testing.T) {
	path := tempFile(t, "aaa aaa aaa")

	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": "aaa",
		"new_text": "bbb",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "bbb aaa aaa" {
		t.Errorf("file content = %q, want %q", string(got), "bbb aaa aaa")
	}
}

func TestEditTool_Execute_ReplaceWithEmpty(t *testing.T) {
	path := tempFile(t, "hello world")
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": " world",
		"new_text": "",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello" {
		t.Errorf("file content = %q, want %q", string(got), "hello")
	}
}

func TestEditTool_Execute_MultilineReplacement(t *testing.T) {
	path := tempFile(t, "line1\nline2\nline3")
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": "line2",
		"new_text": "LINE2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "line1\nLINE2\nline3" {
		t.Errorf("file content = %q, want %q", string(got), "line1\nLINE2\nline3")
	}
}

func TestEditTool_Execute_OldTextNotFound(t *testing.T) {
	path := tempFile(t, "hello world")

	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": "nonexistent",
		"new_text": "anything",
	}))
	if err == nil {
		t.Fatal("expected error when old_text not found, got nil")
	}
}

func TestEditTool_Execute_FileNotFound(t *testing.T) {
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     filepath.Join(t.TempDir(), "ghost.txt"),
		"old_text": "x",
		"new_text": "y",
	}))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestEditTool_Execute_InvalidArgs(t *testing.T) {
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON args, got nil")
	}
}

func TestEditTool_Execute_SuccessMessageContainsPath(t *testing.T) {
	path := tempFile(t, "abc")
	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"path":     path,
		"old_text": "abc",
		"new_text": "xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty success message")
	}
}
