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
//
// session 参数可为 nil——为 nil 时退化为无持久化的无状态模式（兼容旧调用方）。
// 非 nil 时，每轮对话（user / assistant / tool_result）都会追加到 JSONL 文件。
func AgentLoop(
	ctx context.Context,
	client *openai.Client,
	model string,
	instructions string,
	toolContainer *tools.ToolContainer,
	systemPrompt string,
	session *SessionManager,
) error {
	// ── 构造初始消息列表 ───────────────────────────────────────────────────────
	// 若有已有 session，先用历史消息作为基础，再追加 system + 本轮 user 消息。
	// system prompt 始终放在最前面，确保模型上下文优先级正确。
	var messages []openai.ChatCompletionMessageParamUnion
	messages = append(messages, openai.SystemMessage(systemPrompt))

	if session != nil {
		// 续聊：将历史对话插入到 system 之后、本轮 user 之前
		messages = append(messages, session.BuildMessages()...)
	}

	// 记录并追加本轮用户消息
	messages = append(messages, openai.UserMessage(instructions))
	if session != nil {
		if err := session.AddUserEntry(instructions); err != nil {
			WarnSessionPersist("user entry", err)
		}
	}

	// ── 构造工具参数列表 ───────────────────────────────────────────────────────
	paramList := []openai.ChatCompletionToolParam{}
	for _, tool := range toolContainer.ToolMap {
		paramList = append(paramList, tools.ToOpenAiToolParam(tool.Definition()))
	}

	// ── Agent 循环 ─────────────────────────────────────────────────────────────
	for range MaxIterations {
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
			PrintAssistant(msg.Content)
		}

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

		for _, tc := range msg.ToolCalls {
			PrintToolCall(tc.Function.Name, tc.Function.Arguments)
			targetTool, exist := toolContainer.ToolMap[tc.Function.Name]

			if !exist {
				errMsg := fmt.Sprintf("error: unknown tool '%s'", tc.Function.Name)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				continue
			}

			rawParam := json.RawMessage{}
			if err := rawParam.UnmarshalJSON([]byte(tc.Function.Arguments)); err != nil {
				errMsg := fmt.Sprintf("error: function tool can't unmarshalJSON '%s'", tc.Function.Arguments)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				continue
			}

			output, err := targetTool.Execute(ctx, rawParam)
			if err != nil {
				errMsg := fmt.Sprintf("error: %v", err)
				messages = append(messages, openai.ToolMessage(errMsg, tc.ID))
				if session != nil {
					_ = session.AddToolResultEntry(tc.ID, tc.Function.Name, errMsg)
				}
				continue
			}

			messages = append(messages, openai.ToolMessage(output.Content, tc.ID))
			if session != nil {
				if err := session.AddToolResultEntry(tc.ID, tc.Function.Name, output.Content); err != nil {
					WarnSessionPersist("tool result", err)
				}
			}
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
