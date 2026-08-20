package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// BashTool a tool that can execute bash command
type BashTool struct {
	Command string `json:"command"`
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
	return ToolDefinition{
		Name:       "bashTool",
		Desc:       "a tool that can execute bash command",
		Parameters: param,
	}
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
