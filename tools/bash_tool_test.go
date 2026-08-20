package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ─── BashTool tests ───────────────────────────────────────────────────────────

func TestBashTool_Definition(t *testing.T) {
	tool := &BashTool{}
	def := tool.Definition()

	if def.Name != "bash" {
		t.Errorf("Name = %q, want %q", def.Name, "bash")
	}
	if def.Desc == "" {
		t.Error("expected non-empty Desc")
	}
	if _, ok := def.Parameters.Properties["command"]; !ok {
		t.Error("Parameters.Properties should contain 'command'")
	}
	if len(def.Parameters.Required) == 0 || def.Parameters.Required[0] != "command" {
		t.Error("'command' should be in Required")
	}
}

func TestBashTool_Definition_HasGuidelines(t *testing.T) {
	def := (&BashTool{}).Definition()
	if len(def.Guidelines) == 0 {
		t.Error("expected at least one guideline")
	}
}

func TestBashTool_Execute_SimpleEcho(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "echo hello",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Errorf("Content = %q, want it to contain 'hello'", result.Content)
	}
}

func TestBashTool_Execute_CapturesStdout(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "printf 'line1\nline2\n'",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "line1") || !strings.Contains(result.Content, "line2") {
		t.Errorf("Content = %q, expected both lines", result.Content)
	}
}

func TestBashTool_Execute_CapturesStderr(t *testing.T) {
	tool := &BashTool{}
	// A command that writes to stderr and exits non-zero.
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "echo err_msg >&2; exit 1",
	}))
	if err == nil {
		t.Fatal("expected error for non-zero exit code, got nil")
	}
	// The error message should contain the stderr output.
	if !strings.Contains(err.Error(), "err_msg") {
		t.Errorf("error = %v, expected it to contain stderr output 'err_msg'", err)
	}
}

func TestBashTool_Execute_NonZeroExit_ReturnsError(t *testing.T) {
	tool := &BashTool{}
	_, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "exit 2",
	}))
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
}

func TestBashTool_Execute_ChainedCommands(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "echo a && echo b",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "a") || !strings.Contains(result.Content, "b") {
		t.Errorf("Content = %q, expected both 'a' and 'b'", result.Content)
	}
}

func TestBashTool_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tool := &BashTool{}
	_, err := tool.Execute(ctx, mustMarshal(t, map[string]string{
		"command": "sleep 10",
	}))
	if err == nil {
		t.Fatal("expected error due to cancelled context, got nil")
	}
}

func TestBashTool_Execute_InvalidArgs(t *testing.T) {
	tool := &BashTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON args, got nil")
	}
}

func TestBashTool_Execute_EnvVarAccess(t *testing.T) {
	t.Setenv("TEST_BASH_VAR", "hello_from_env")
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), mustMarshal(t, map[string]string{
		"command": "echo $TEST_BASH_VAR",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "hello_from_env") {
		t.Errorf("Content = %q, expected env var value", result.Content)
	}
}
