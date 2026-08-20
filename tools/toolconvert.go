package tools

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

func ToOpenAiToolParam(toolDefintion ToolDefinition) openai.ChatCompletionToolParam {

	param := map[string]any{}
	for k, v := range toolDefintion.Parameters.Properties {
		param[k] = v
	}

	openAiToolParam := openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        toolDefintion.Name,
			Description: openai.String(toolDefintion.Desc),

			Parameters: shared.FunctionParameters{
				"type":                 toolDefintion.Parameters.Type,
				"properties":           param,
				"required":             toolDefintion.Parameters.Required,
				"additionalProperties": false,
			},
		},
	}

	return openAiToolParam
}
