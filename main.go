package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

const maxIterations = 20

func main() {
	apiKey := os.Getenv("LLM_TOKEN")
	if apiKey == "" {
		panic("LLM_TOKEN environment variable is not set")
	}
	baseURL := os.Getenv("LLM_END_PROT")
	if baseURL == "" {
		panic("LLM_END_PROT environment variable is not set")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	tool := bashTool()

	err := agentLoop(
		context.Background(),
		&client,
		"claude-sonnet-4-6",
		"请评价我写的 AgentLoop 代码并提出改进建议，你可以直接修改代码。",
		[]openai.ChatCompletionToolParam{tool},
	)
	if err != nil {
		panic(err)
	}
}

// agentLoop 运行一个 Chat Completions 风格的 Agent 循环。
// 模型可以选择返回文本或调用工具，循环持续直到模型给出最终回复或达到最大迭代次数。
func agentLoop(
	ctx context.Context,
	client *openai.Client,
	model string,
	instructions string,
	tools []openai.ChatCompletionToolParam,
) error {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant that can use tools to answer questions. " +
			"When you need to execute a bash command, use the execute_bash_command tool."),
		openai.UserMessage(instructions),
	}

	for i := range maxIterations {
		fmt.Printf("=== iteration %d ===\n", i+1)

		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    shared.ChatModel(model),
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return fmt.Errorf("chat completion error: %w", err)
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// 打印模型的文本回复（如果有）
		if msg.Content != "" {
			fmt.Printf("assistant: %s\n", msg.Content)
		}

		switch choice.FinishReason {
		case "stop", "length":
			// 模型已给出最终回复，正常结束
			return nil

		case "tool_calls":
			// 将 assistant 消息（含 tool_calls）追加到历史记录
			messages = append(messages, msg.ToParam())

			// 执行每个工具调用
			for _, tc := range msg.ToolCalls {
				fmt.Printf("tool call: %s(%s)\n", tc.Function.Name, tc.Function.Arguments)

				if tc.Function.Name != "execute_bash_command" {
					messages = append(messages, openai.ToolMessage(
						fmt.Sprintf("error: unknown tool '%s'", tc.Function.Name),
						tc.ID,
					))
					continue
				}

				var args struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					messages = append(messages, openai.ToolMessage(
						fmt.Sprintf("error: failed to parse arguments: %v", err),
						tc.ID,
					))
					continue
				}

				output, err := executeBashCommand(args.Command)
				if err != nil {
					messages = append(messages, openai.ToolMessage(
						fmt.Sprintf("error: %v", err),
						tc.ID,
					))
					continue
				}

				messages = append(messages, openai.ToolMessage(output, tc.ID))
			}

		default:
			// content_filter 等意外情况，安全结束
			return nil
		}
	}

	return fmt.Errorf("agent loop exceeded max iterations (%d)", maxIterations)
}

// bashTool 返回一个可执行 bash 命令的 function calling 工具定义。
func bashTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        "execute_bash_command",
			Description: openai.String("Execute a bash command and return the output."),
			Parameters: shared.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "A bash command to execute",
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
		},
	}
}

// executeBashCommand 执行一条 bash 命令并返回 stdout+stderr 的合并输出。
func executeBashCommand(command string) (string, error) {
	out, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\noutput: %s", err, string(out))
	}
	return string(out), nil
}
