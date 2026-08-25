package engine

import (
	"axon/tools"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

const MaxIterations = 2000

// AgentLoop runs a Chat Completions-style agent loop.
// The model may return text, call tools, or both; the loop continues until the
// model produces a final reply or MaxIterations is reached.
func AgentLoop(
	ctx context.Context,
	client *openai.Client,
	model Model,
	instructions string,
	toolContainer *tools.ToolContainer,
	systemPrompt string,
	session *SessionManager,
) error {
	// ── 构造初始消息列表 ───────────────────────────────────────────────────────
	// 若有已有 session，先用历史消息作为基础，再追加 system + 本轮 user 消息。
	// system prompt 始终放在最前面，确保模型上下文优先级正确。
	if session == nil {
		return fmt.Errorf("param error,session param can't be nil")
	}

	// 先持久化当前 user，再从 session 统一构造请求上下文。
	if err := session.AddUserEntry(instructions); err != nil {
		WarnSessionPersist("user entry", err)
	}
	messages := buildAgentMessages(systemPrompt, session)

	// ── 构造工具参数列表 ───────────────────────────────────────────────────────
	paramList := []openai.ChatCompletionToolParam{}
	for _, tool := range toolContainer.ToolMap {
		paramList = append(paramList, tools.ToOpenAiToolParam(tool.Definition()))
	}

	// ── Agent 循环 ─────────────────────────────────────────────────────────────
	for range MaxIterations {
		//TODO pre check extesion point
		err := checkCompactionAndCompaction(ctx, session, model, client)
		if err != nil {
			return err
		}
		// compaction 可能改变了当前 context，必须丢弃旧 messages 并重建。
		messages = buildAgentMessages(systemPrompt, session)

		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    shared.ChatModel(model.ID),
			Messages: messages,
			Tools:    paramList,
		})
		if err != nil {
			return fmt.Errorf("chat completion error: %w", err)
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// ── length 截断处理 ──────────────────────────────────────────────────
		// 当响应因 token 上限被截断且有未执行的 tool_call 时，
		// 不能静默完成——把所有 tool_call 标记为失败塞回，让模型重发。
		if choice.FinishReason == "length" && len(msg.ToolCalls) != 0 {
			WarnResponseTruncated()
			messages = append(messages, msg.ToParam())

			// 持久化截断的 assistant 消息（内容可能不完整）
			if session != nil {
				stcs := toSessionToolCalls(msg.ToolCalls)
				if err := session.AddAssistantEntry(msg.Content, stcs); err != nil {
					WarnSessionPersist("truncated assistant entry", err)
				}
			}

			for _, tc := range msg.ToolCalls {
				errMsg := fmt.Sprintf(
					"Tool call %s was not executed: the response hit the output token limit, "+
						"so its arguments may be truncated. Re-issue the tool call with complete arguments.",
					tc.Function.Name,
				)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					if err := session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg); err != nil {
						WarnSessionPersist("truncation tool result", err)
					}
				}
			}
			// 继续循环，让模型重发完整的 tool_call
			continue
		}

		// ── 无工具调用：最终回复，退出循环 ──────────────────────────────────
		if len(msg.ToolCalls) == 0 {
			PrintAssistant(msg.Content)
			// 持久化最终 assistant 回复
			if session != nil {
				if err := session.AddAssistantEntry(msg.Content, nil); err != nil {
					WarnSessionPersist("final assistant entry", err)
				}
			}
			return nil
		}

		// ── 有工具调用：执行并将结果塞回 ────────────────────────────────────
		messages = append(messages, msg.ToParam())

		// 持久化 assistant 消息（含 tool_calls）
		if session != nil {
			stcs := toSessionToolCalls(msg.ToolCalls)
			if err := session.AddAssistantEntry(msg.Content, stcs); err != nil {
				WarnSessionPersist("assistant entry", err)
			}
		}

		PrintThinking(msg.Content)

		for _, tc := range msg.ToolCalls {
			PrintToolCall(tc.Function.Name, tc.Function.Arguments)
			targetTool, exist := toolContainer.ToolMap[tc.Function.Name]

			if !exist {
				errMsg := fmt.Sprintf("error: unknown tool '%s'", tc.Function.Name)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				PrintToolResult(errMsg)
				continue
			}

			rawParam := json.RawMessage{}
			if err := rawParam.UnmarshalJSON([]byte(tc.Function.Arguments)); err != nil {
				errMsg := fmt.Sprintf("error: function tool can't unmarshalJSON '%s'", tc.Function.Arguments)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				PrintToolResult(errMsg)
				continue
			}

			output, err := targetTool.Execute(ctx, rawParam)
			if err != nil {
				errMsg := fmt.Sprintf("error: %v", err)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				PrintToolResult(errMsg)
				continue
			}

			messages = append(messages, openai.ToolMessage(output.Content, tc.ID))
			if session != nil {
				if err := session.AddToolResultEntry(tc.ID, tc.Function.Name, output.Content); err != nil {
					WarnSessionPersist("tool result", err)
				}
			}

			displayResult := formatToolResultForCLI(tc.Function.Name, output.Content)
			PrintToolResult(displayResult)
		}
	}

	return fmt.Errorf("agent loop exceeded max iterations (%d)", MaxIterations)
}

// toSessionToolCalls 将 openai SDK 的 tool_call 列表转换为持久化格式。
func toSessionToolCalls(tcs []openai.ChatCompletionMessageToolCall) []SessionToolCall {
	out := make([]SessionToolCall, 0, len(tcs))
	for _, tc := range tcs {
		out = append(out, SessionToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return out
}

func buildAgentMessages(systemPrompt string, session *SessionManager) []openai.ChatCompletionMessageParamUnion {
	messages := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}
	return append(messages, session.BuildMessages()...)
}

func doCompactionInner(ctx context.Context, session *SessionManager, model Model, client *openai.Client) error {
	// 只对最新 compaction 之后的消息做下一轮摘要；旧 summary 已经代表更早历史。
	latestSummary := -1
	for i := session.Len() - 1; i >= 0; i-- {
		if session.sessionList[i].Type == EntryTypeSummary {
			latestSummary = i
			break
		}
	}
	start := latestSummary + 1
	candidates := session.sessionList[start:]
	if len(candidates) < 2 {
		return fmt.Errorf("cannot compact: current input itself is too large")
	}

	// 至少保留最后一条 entry（通常是当前 user 或 tool result），避免摘要吞掉当前 turn。
	summarizeCount := 0
	summarizeTokens := 0
	budget := model.ContextWindow / 2
	if budget <= 0 {
		return fmt.Errorf("cannot compact: invalid context window %d", model.ContextWindow)
	}
	for summarizeCount < len(candidates)-1 {
		next := EstimateEntryTokens(candidates[summarizeCount])
		if summarizeCount > 0 && summarizeTokens+next > budget {
			break
		}
		summarizeTokens += next
		summarizeCount++
	}
	if summarizeCount == 0 {
		return fmt.Errorf("cannot compact: current input itself is too large")
	}

	toSummarize := candidates[:summarizeCount]
	firstKeptEntryID := candidates[summarizeCount].ID
	summaryPrompt := "请仔细阅读下面的会话历史并生成摘要。保留用户目标、背景、关键事实、已经完成的工作、工具操作结果、重要文件和未完成任务；删除重复和不必要的细节。"
	summaryMessages := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(summaryPrompt)}
	summaryMessages = append(summaryMessages, buildMessagesFromEntries(toSummarize)...)

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model.ID),
		Messages: summaryMessages,
	})
	if err != nil {
		return fmt.Errorf("summary completion error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("summary completion returned no choices")
	}
	choice := resp.Choices[0]
	if choice.Message.Content == "" {
		return fmt.Errorf("summary completion returned empty content: finish_reason=%q refusal=%q tool_calls=%d raw=%s", choice.FinishReason, choice.Message.Refusal, len(choice.Message.ToolCalls), choice.RawJSON())
	}

	return session.AddSummaryEntryWithBoundary(choice.Message.Content, firstKeptEntryID)
}

func checkCompactionAndCompaction(ctx context.Context, session *SessionManager, model Model, client *openai.Client) error {
	for session.estimateTokenCnt() >= model.ContextWindow {
		before := session.estimateTokenCnt()
		if err := doCompactionInner(ctx, session, model, client); err != nil {
			return err
		}
		if session.estimateTokenCnt() >= before {
			return fmt.Errorf("compaction did not reduce context: before=%d after=%d", before, session.estimateTokenCnt())
		}
	}
	return nil
}

func formatToolResultForCLI(toolName string, outputContent string) string {
	if toolName == "read" {
		return fmt.Sprintf("(Read %d bytes)", len(outputContent))
	}
	return strings.TrimSpace(outputContent)
}
