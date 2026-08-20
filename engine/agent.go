package engine

import (
	"axon/tools"
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

const MaxIterations = 20

// AgentLoop runs a Chat Completions-style agent loop.
// The model may return text, call tools, or both; the loop continues until the
// model produces a final reply or MaxIterations is reached.
func AgentLoop(
	ctx context.Context,
	client *openai.Client,
	model string,
	instructions string,
	toolContainer *tools.ToolContainer,
	systemPrompt string,
) error {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(instructions),
	}

	paramList := []openai.ChatCompletionToolParam{}
	for _, tool := range toolContainer.ToolMap {
		paramList = append(paramList, tools.ToOpenAiToolParam(tool.Definition()))
	}

	for i := range MaxIterations {
		fmt.Printf("=== iteration %d ===\n", i+1)
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    shared.ChatModel(model),
			Messages: messages,
			Tools:    paramList,
		})
		if err != nil {
			return fmt.Errorf("chat completion error: %w", err)
		}

		choice := resp.Choices[0]
		msg := choice.Message

		if msg.Content != "" {
			fmt.Printf("assistant: %s\n", msg.Content)
		}

		if choice.FinishReason == "length" && len(msg.ToolCalls) != 0 {
			// TODO: when multi-tool support lands, verify this truncation path carefully.
			fmt.Println("[warning] response truncated with pending tool calls")
			messages = append(messages, msg.ToParam())
			for _, tc := range msg.ToolCalls {
				messages = append(messages, openai.ToolMessage(
					fmt.Sprintf("Tool call %s was not executed: the response hit the output token limit, so its arguments may be truncated. Re-issue the tool call with complete arguments.", tc.Function.Name),
					tc.ID,
				))
			}
			return nil
		}

		if len(msg.ToolCalls) == 0 {
			return nil
		}

		messages = append(messages, msg.ToParam())

		for _, tc := range msg.ToolCalls {
			fmt.Printf("tool call: %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			targetTool, exist := toolContainer.ToolMap[tc.Function.Name]

			if !exist {
				messages = append(messages, openai.ToolMessage(
					fmt.Sprintf("error: unknown tool '%s'", tc.Function.Name),
					tc.ID,
				))
				continue
			}

			rawParam := json.RawMessage{}
			if err := rawParam.UnmarshalJSON([]byte(tc.Function.Arguments)); err != nil {
				messages = append(messages, openai.ToolMessage(
					fmt.Sprintf("error: function tool can't unmarshalJSON '%s'", tc.Function.Arguments),
					tc.ID,
				))
				continue
			}

			output, err := targetTool.Execute(ctx, rawParam)
			if err != nil {
				messages = append(messages, openai.ToolMessage(
					fmt.Sprintf("error: %v", err),
					tc.ID,
				))
				continue
			}

			messages = append(messages, openai.ToolMessage(output.Content, tc.ID))
		}
	}

	return fmt.Errorf("agent loop exceeded max iterations (%d)", MaxIterations)
}
