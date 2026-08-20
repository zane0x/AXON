package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ─── ReadTool ────────────────────────────────────────────────────────────────

// ReadTool reads the full contents of a file from disk.
type ReadTool struct {
	Path string `json:"path"`
}

func (r *ReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "read",
		Desc: "Read the contents of a file at the given path and return them as a string. " +
			"Use this to inspect source code, configuration files, logs, or any text-based file " +
			"before making edits or drawing conclusions about its content. " +
			"This tool is read-only and does not modify the file in any way.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to read. Relative paths are resolved from the current working directory.",
				},
			},
			Required: []string{"path"},
		},
		Snippet: "Read the contents of a file at the given path (read-only).",
		Guidelines: []string{
			"Always read a file before editing it to ensure old_text matches exactly.",
			"Use relative paths when the file is inside the project directory; use absolute paths only when necessary.",
			"For large files, consider using the bash tool with `grep` or `sed` to extract only the relevant section.",
			"Do NOT attempt to read binary files (images, compiled binaries, etc.) — the output will be unreadable.",
			"If the file does not exist, the tool returns an error; verify the path with bash (`ls`) first when uncertain.",
		},
	}
}

func (r *ReadTool) Execute(_ context.Context, args json.RawMessage) (ToolResult, error) {
	if err := json.Unmarshal(args, r); err != nil {
		return ToolResult{}, fmt.Errorf("read: failed to parse args: %w", err)
	}
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read: %w", err)
	}
	return ToolResult{Content: string(data)}, nil
}

// ─── WriteTool ───────────────────────────────────────────────────────────────

// WriteTool writes content to a file, creating or fully overwriting it.
type WriteTool struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "write",
		Desc: "Write content to a file at the given path, creating the file if it does not exist or " +
			"overwriting it entirely if it does. " +
			"Use this when you need to create a new file or replace a file's content wholesale. " +
			"For targeted, surgical changes to an existing file prefer the edit tool instead, " +
			"to avoid accidentally discarding content.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to write. Parent directories must already exist.",
				},
				"content": {
					Type:        "string",
					Description: "The full content to write into the file. Existing content is completely replaced.",
				},
			},
			Required: []string{"path", "content"},
		},
		Snippet: "Write content to a file, creating or fully overwriting it.",
		Guidelines: []string{
			"Prefer edit over write when only a small portion of an existing file needs to change.",
			"Always include a trailing newline in text files to comply with POSIX conventions.",
			"Read the file first (with the read tool) if you are unsure of its current content, so nothing is lost.",
			"Ensure the parent directory exists before writing; create it with bash (`mkdir -p dir/`) if needed.",
			"Do NOT write binary content — this tool writes the content string as-is in UTF-8.",
			"When writing code files, preserve the correct indentation style (tabs vs spaces) already used in the project.",
		},
	}
}

func (w *WriteTool) Execute(_ context.Context, args json.RawMessage) (ToolResult, error) {
	if err := json.Unmarshal(args, w); err != nil {
		return ToolResult{}, fmt.Errorf("write: failed to parse args: %w", err)
	}
	if err := os.WriteFile(w.Path, []byte(w.Content), 0644); err != nil {
		return ToolResult{}, fmt.Errorf("write: %w", err)
	}
	return ToolResult{Content: fmt.Sprintf("Successfully wrote %d bytes to %s", len(w.Content), w.Path)}, nil
}

// ─── EditTool ────────────────────────────────────────────────────────────────

// EditTool performs a targeted find-and-replace on an existing file.
type EditTool struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (e *EditTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "edit",
		Desc: "Edit a file by replacing the first occurrence of old_text with new_text. " +
			"Use this for precise, surgical changes to an existing file without touching the rest of its content. " +
			"old_text must match the file's content character-for-character, including whitespace and line endings. " +
			"If old_text appears more than once, only the first occurrence is replaced.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to edit.",
				},
				"old_text": {
					Type:        "string",
					Description: "The exact text to search for and replace. Must match the file content verbatim, including indentation and newlines.",
				},
				"new_text": {
					Type:        "string",
					Description: "The text to substitute in place of old_text.",
				},
			},
			Required: []string{"path", "old_text", "new_text"},
		},
		Snippet: "Replace the first occurrence of old_text with new_text in a file.",
		Guidelines: []string{
			"Always read the file first to copy old_text exactly — even one mismatched space will cause the edit to fail.",
			"Include enough surrounding context in old_text to make it unique within the file.",
			"Prefer edit over write when the change is small relative to the overall file size.",
			"To make multiple non-overlapping changes, call edit sequentially — each call re-reads the updated file.",
			"If old_text is not found, the tool returns an error; verify the content with read or bash before retrying.",
			"Do not use edit to rename or move files — use bash (`mv`) for that instead.",
		},
	}
}

func (e *EditTool) Execute(_ context.Context, args json.RawMessage) (ToolResult, error) {
	if err := json.Unmarshal(args, e); err != nil {
		return ToolResult{}, fmt.Errorf("edit: failed to parse args: %w", err)
	}

	data, err := os.ReadFile(e.Path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("edit: read file: %w", err)
	}

	original := string(data)
	if !strings.Contains(original, e.OldText) {
		return ToolResult{}, fmt.Errorf("edit: old_text not found in %s", e.Path)
	}

	updated := strings.Replace(original, e.OldText, e.NewText, 1)
	if err := os.WriteFile(e.Path, []byte(updated), 0644); err != nil {
		return ToolResult{}, fmt.Errorf("edit: write file: %w", err)
	}

	return ToolResult{Content: fmt.Sprintf("Successfully edited %s", e.Path)}, nil
}
