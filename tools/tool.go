package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type ToolDefinition struct {
	Name       string
	Desc       string
	Parameters ParameterSchema
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

// BashTool  a tool that can execute bash command
type BashTool struct {
	Command string `json:command`
}

func (b *BashTool) Definition() ToolDefinition {

	param := ParameterSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"command": {
				Type:        "string",
				Description: "the command that want bash tool execute",
			},
		},
		Required: []string{"command"},
	}
	define := ToolDefinition{
		Name:       "bashTool",
		Desc:       "a tool that can execute bash command",
		Parameters: param,
	}

	return define
}

func (b *BashTool) Execute(ctx context.Context, args json.RawMessage) (ToolResult, error) {
	result := ToolResult{}
	jsonBtye, err := args.MarshalJSON()
	if err != nil {
		return result, fmt.Errorf("args marshal from json error,err:%w,args:%s", err, string(jsonBtye))
	}

	err = json.Unmarshal(jsonBtye, b)
	if err != nil {
		return result, fmt.Errorf("args marshal from json error,err:%w,args:%s", err, string(jsonBtye))
	}

	ret, err := exec.CommandContext(ctx, "bash", "-c", b.Command).CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("command failed: %w\noutput: %s", err, string(ret))
	}
	result.Content = string(ret)
	return result, nil
}
