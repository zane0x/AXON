package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ─── ReadTool ────────────────────────────────────────────────────────────────

type ReadTool struct {
	Path string `json:"path"`
}

func (r *ReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "read",
		Desc: "Read the contents of a file at the given path (read-only).",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to read.",
				},
			},
			Required: []string{"path"},
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

type WriteTool struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "write",
		Desc: "Write content to a file at the given path, overwriting any existing content (write-only).",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to write.",
				},
				"content": {
					Type:        "string",
					Description: "The content to write into the file.",
				},
			},
			Required: []string{"path", "content"},
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

type EditTool struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (e *EditTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "edit",
		Desc: "Edit a file by replacing the first occurrence of old_text with new_text.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path": {
					Type:        "string",
					Description: "The path of the file to edit.",
				},
				"old_text": {
					Type:        "string",
					Description: "The exact text to search for and replace.",
				},
				"new_text": {
					Type:        "string",
					Description: "The text to replace old_text with.",
				},
			},
			Required: []string{"path", "old_text", "new_text"},
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
