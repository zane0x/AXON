package main

import (
	"axon/engine"
	"axon/tools"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

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
		fmt.Fprintln(os.Stderr, "LLM_TOKEN env variable not set, exit")
		os.Exit(1)
	}
	baseURL := os.Getenv("LLM_END_PROT")
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "LLM_END_PROT env variable not set, exit")
		os.Exit(1)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	systemPrompt := engine.BuildSystemPrompt(engine.SystemPromptOptions{
		Cwd:       cwd,
		Container: toolContainer,
	})

	// REPL
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, err := reader.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			os.Exit(1)
		}

		prompt := strings.TrimSpace(text)
		if prompt == "exit" {
			fmt.Println("Bye~Bye~")
			return
		}

		err = engine.AgentLoop(ctx, &client, "claude-sonnet-4-6", prompt, toolContainer, systemPrompt)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Print("\ninterrupted")
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "agent error: %v\n", err)
			os.Exit(1)
		}
	}
}
