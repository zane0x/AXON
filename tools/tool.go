package tools

import (
	"context"
	"encoding/json"
)

type ToolDefinition struct {
	Name       string
	Desc       string
	Parameters ParameterSchema
	Snippet    string
	Guidelines []string
}

type ParameterSchema struct {
	Type       string
	Properties map[string]PropertySchema
	Required   []string
}

type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type AgentTool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (ToolResult, error)
}

type ToolResult struct {
	Content string
}

type ToolContainer struct {
	ToolMap map[string]AgentTool
}

func NewToolContainer() *ToolContainer {
	return &ToolContainer{
		ToolMap: make(map[string]AgentTool),
	}
}

func (t *ToolContainer) RegisterTool(tool AgentTool) {
	t.ToolMap[tool.Definition().Name] = tool
}
