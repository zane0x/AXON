package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

func AgentLoop(ctx context.Context, client *openai.Client, model string, instructions string, tools []responses.ToolUnionParam) error {

	systemPrompt := fmt.Sprintf(`You are a helpful assistant that can use tools to answer questions. You have access to the following tools: %v`, tools)

	var inputParam []responses.ResponseInputItemUnionParam

	inputParam = append(inputParam, responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role: "assistant",
			Content: []responses.ResponseInputContentUnionParam{
				{
					OfInputText: &responses.ResponseInputTextParam{
						Text: systemPrompt,
					},
				},
			},
		},
	},
	)

	inputParam = append(inputParam, responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role: "user",
			Content: []responses.ResponseInputContentUnionParam{
				{
					OfInputText: &responses.ResponseInputTextParam{
						Text: instructions,
					},
				},
			},
		},
	},
	)

	var resp []string
	var lastResp []responses.ResponseInputItemUnionParam

	i := 0
	for {
		i += 1
		fmt.Printf("current loop is %d\n", i)
		if len(lastResp) > 0 {
			for _, r := range lastResp {
				inputParam = append(inputParam, r)
			}
		}
		if len(resp) > 0 {
			for _, r := range resp {
				inputParam = append(inputParam, responses.ResponseInputItemUnionParam{
					OfInputMessage: &responses.ResponseInputItemMessageParam{
						Role: "function",
						Content: []responses.ResponseInputContentUnionParam{
							{
								OfInputText: &responses.ResponseInputTextParam{
									Text: r,
								},
							},
						},
					},
				})
			}
		}

		response, err := client.Responses.New(ctx, responses.ResponseNewParams{
			Model: model,
			Input: responses.ResponseNewParamsInputUnion{OfInputItemList: inputParam},
			Tools: tools,
		})
		if err != nil {
			fmt.Printf("client.Responses.New error: %v,current loop is ending", err)
			return err
		}

		isFinish, outputStr, lastOutPutArr, functionCalls, err := ParseLLMResponse(response)
		if err != nil {
			fmt.Printf("ParseLLMResponse error: %v,current loop is ending", err)
			return err
		}
		lastResp = lastOutPutArr
		//fmt.Printf("output:%s", outputStr)
		if isFinish {
			fmt.Printf("current session is finished, output is:%s", outputStr)
			return nil
		}
		resp = functionCall(functionCalls, tools)
	}

}

func functionCall(functionCalls []*responses.ResponseFunctionToolCall, tools []responses.ToolUnionParam) []string {

	//把tools转成map，方便根据functionCall的name找到对应的tool
	toolMap := make(map[string]responses.ToolUnionParam)
	for _, tool := range tools {
		if tool.OfFunction != nil && tool.OfFunction.Name != "" {
			toolMap[tool.OfFunction.Name] = tool
		}
	}
	var functionCallsResponse []string
	for _, functionCall := range functionCalls {
		tool, ok := toolMap[functionCall.Name]
		if !ok {
			fmt.Printf("tool %s not found", functionCall.Name)
			continue
		}
		if tool.OfFunction.Name != "execute_bash_command" {
			fmt.Printf("tool %s is not execute_bash_command", tool.OfFunction.Name)
			continue
		}
		if tool.OfFunction != nil && tool.OfFunction.Name == "execute_bash_command" {
			var arguments struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(functionCall.Arguments), &arguments); err != nil {
				fmt.Printf("failed to unmarshal arguments: %v", err)
				continue
			}
			output, err := executeBashCommand(arguments.Command)
			if err != nil {
				fmt.Printf("failed to execute bash command: %v", err)
				continue
			}

			//fmt.Printf("function call result - callId:%s,callName:%s,arguments:%s,result:%s\n", functionCall.CallID, functionCall.Name, functionCall.Arguments, output)

			functionCallsResponse = append(functionCallsResponse, output)
		}

	}
	return functionCallsResponse
}

func ParseLLMResponse(
	response *responses.Response,
) (bool, string, []responses.ResponseInputItemUnionParam, []*responses.ResponseFunctionToolCall, error) {

	var respToInputArr []responses.ResponseInputItemUnionParam
	var outputFunctionCall []*responses.ResponseFunctionToolCall

	// Response 本身失败 / 取消，直接结束
	switch response.Status {
	case "failed", "cancelled", "incomplete":
		fmt.Printf("current response status: %s\n", response.Status)
		return true, "", nil, nil, nil

	case "completed":
		// 正常，继续解析 output
	default:
		// queued / in_progress 等状态，暂时认为还没结束
		return false, "", nil, nil, nil
	}

	for _, output := range response.Output {

		switch output.Type {

		case "function_call":
			// 模型要求调用工具
			fc := output.AsFunctionCall()
			outputFunctionCall = append(outputFunctionCall, &fc)
			// 把模型的 function_call 保存到下一轮 history
			respToInputArr = append(
				respToInputArr,
				responses.ResponseInputItemUnionParam{
					OfFunctionCall: &responses.ResponseFunctionToolCallParam{
						CallID:    fc.CallID,
						Name:      fc.Name,
						Arguments: fc.Arguments,
					},
				},
			)
			fmt.Printf("function call - callId:%s,callName:%s,arguments:%s\n", fc.CallID, fc.Name, fc.Arguments)
		case "message":
			return true, response.OutputText(), respToInputArr, outputFunctionCall, nil

		default:
			// reasoning 等暂时先忽略
			fmt.Printf("output type:%s output:%+v\n", output.Type, output.JSON.Summary)
		}
	}

	// 有 function call -> Agent Loop 继续
	if len(outputFunctionCall) > 0 {
		return false, "", respToInputArr, outputFunctionCall, nil
	}

	// 没有 function call -> 认为模型已经给出最终答案
	return true, response.OutputText(), respToInputArr, nil, nil
}

func main() {
	client := openai.NewClient(
		option.WithAPIKey("sk-q51CvEd12XVcnejinhBfVDKilFVNJQ0v1HBFkAYEv1CW51Zm"),
		option.WithBaseURL("https://new.xkool.cfd/v1"),
		option.WithMiddleware(func(r *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			// new.xkool.cfd 网关屏蔽 openai 官方 SDK 的 User-Agent，伪装成 curl
			r.Header.Set("User-Agent", "curl/8.5.0")
			return next(r)
		}),
	)
	tool := bashTool()
	err := AgentLoop(context.Background(), &client, "glm-5.1", "这个文件/home/work/code/zyron/main.go 是我写的一个简单的agentLoop，你来评价一下我写的这个AgentLoop.然后按照你的建议来对这个文件进行修改，让他符合最佳实践。你修改后的代码可以叫main_opt.go。且你实现的这个最佳实践需要通过自测没问题再交付", []responses.ToolUnionParam{tool})
	if err != nil {
		panic(err)
	}
}

/**
 * 下面是一个为大模型function call提供的可以执行bash命令的工具
 **/

func bashTool() responses.ToolUnionParam {
	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "A bash command to execute"},
		},
		"required":             []string{"command"},
		"additionalProperties": false,
	}
	tool := responses.ToolParamOfFunction("execute_bash_command", parameters, false)
	tool.OfFunction.Description = openai.String("Execute a bash command and return the output.")
	return tool
}

/**
 * 下面是一个为大模型function call提供的可以执行bash命令的工具
 **/
func executeBashCommand(command string) (string, error) {
	osCommand := exec.Command("bash", "-c", command)
	output, err := osCommand.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v", err)
	}
	return string(output), nil
}
