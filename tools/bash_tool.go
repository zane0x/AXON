package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// BashTool executes arbitrary shell commands in a bash subprocess.
type BashTool struct {
	Command string `json:"command"`
}

func (b *BashTool) Definition() ToolDefinition {
	param := ParameterSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"command": {
				Type:        "string",
				Description: "The bash command to execute. Must be a non-empty, valid shell expression.",
			},
		},
		Required: []string{"command"},
	}
	return ToolDefinition{
		Name: "bash",
		Desc: "Execute a bash command in a subprocess and return its combined stdout and stderr output. " +
			"Use for running scripts, inspecting the filesystem, installing packages, building projects, " +
			"or any task that requires shell access. " +
			"The command runs with the same working directory and environment as the parent process. " +
			"Prefer targeted, read-safe commands; avoid destructive or irreversible operations without explicit user confirmation.",
		Parameters: param,
		Snippet:    "Execute a bash command and return its combined stdout/stderr output.",
		Guidelines: []string{
			"Chain multiple steps with && or ; rather than making separate tool calls when the steps are logically related.",
			"Always quote file paths that may contain spaces: e.g. `cat \"path/to/my file.txt\"`.",
			"Prefer non-destructive inspection commands (ls, cat, grep, find) before making changes.",
			"When output may be large, pipe through head, tail, or grep to limit results: e.g. `some_cmd | head -100`.",
			"Do NOT run commands that start background daemons or open interactive prompts (e.g. vim, less, top) — they will hang.",
			"Do NOT use sudo or commands that require elevated privileges unless explicitly authorised by the user.",
			"If a command is expected to take a long time, inform the user before invoking it.",
			"Capture stderr alongside stdout (CombinedOutput) so errors are visible in the result.",
		},
	}
}

func (b *BashTool) Execute(ctx context.Context, args json.RawMessage) (ToolResult, error) {
	result := ToolResult{}
	jsonByte, err := args.MarshalJSON()
	if err != nil {
		return result, fmt.Errorf("bash: failed to marshal args: %w", err)
	}

	if err = json.Unmarshal(jsonByte, b); err != nil {
		return result, fmt.Errorf("bash: failed to parse args: %w, raw: %s", err, string(jsonByte))
	}

	ret, err := exec.CommandContext(ctx, "bash", "-c", b.Command).CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("bash: command failed: %w\noutput: %s", err, string(ret))
	}
	result.Content = string(ret)
	return result, nil
}
