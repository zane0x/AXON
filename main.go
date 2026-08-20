package main

import (
	"axon/engine"
	"axon/tools"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

const maxIterations = 20

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// resolve cwd as the working directory from which the binary was invoked
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
		os.Exit(1)
	}

	// tool init
	toolContainer := tools.NewToolContainer()

	toolContainer.RegisterTool(&tools.BashTool{})
	toolContainer.RegisterTool(&tools.ReadTool{})
	toolContainer.RegisterTool(&tools.WriteTool{})
	toolContainer.RegisterTool(&tools.EditTool{})

	// init llm client
	apiKey := os.Getenv("LLM_TOKEN")
	if apiKey == "" {
		fmt.Println("can't find LLM_TOKEN env variable,exit")
		os.Exit(1)
	}
	baseURL := os.Getenv("LLM_END_PROT")
	if baseURL == "" {
		fmt.Println("can't find LLM_END_PROT env variable,exit")
		os.Exit(1)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	// run agent loop
	for {
		fmt.Print(">")
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		prompt := strings.TrimSpace(text)
		if prompt == "exit" {
			fmt.Print("Bye~Bye~\n")
			return
		}

		err = agentLoop(
			ctx,
			&client,
			"claude-sonnet-4-6",
			prompt,
			toolContainer,
			engine.BuildSystemPrompt(*toolContainer, cwd),
		)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Print("\ninterrupted")
				os.Exit(130)
			}
			panic(err)
		}
	}

}

// agentLoop 运行一个 Chat Completions 风格的 Agent 循环。
// 模型可以选择返回文本或调用工具，循环持续直到模型给出最终回复或达到最大迭代次数。
func agentLoop(
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

	for i := range maxIterations {
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

		// 打印模型的文本回复（如果有）
		if msg.Content != "" {
			fmt.Printf("assistant: %s\n", msg.Content)
		}

		if choice.FinishReason == "length" && len(msg.ToolCalls) != 0 {
			//TODO 支持多工具场景的时候，应该着重检查此场景。如果正好在这里返回了tool_call，应该执行fastFail..
			fmt.Println("[warning] response truncated with pending tool calls")

			messages = append(messages, msg.ToParam())

			for _, tc := range msg.ToolCalls {
				messages = append(messages, openai.ToolMessage(

					fmt.Sprintf("Tool call %s was not executed: the response hit the output token limit, so its arguments may be truncated. Re-issue the tool call with complete arguments.", tc.Function.Name), tc.ID))

			}
			return nil
		}

		if len(msg.ToolCalls) == 0 {
			// current turn finish normally..
			return nil
		}

		//tool call
		// 将 assistant 消息（含 tool_calls）追加到历史记录
		messages = append(messages, msg.ToParam())

		// 执行每个工具调用
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

	return fmt.Errorf("agent loop exceeded max iterations (%d)", maxIterations)
}
