package tools

import (
	"testing"
)

// ─── ToOpenAiToolParam tests ──────────────────────────────────────────────────

func TestToOpenAiToolParam_NameAndDesc(t *testing.T) {
	def := ToolDefinition{
		Name: "mytool",
		Desc: "does something useful",
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{},
		},
	}
	param := ToOpenAiToolParam(def)

	if param.Function.Name != "mytool" {
		t.Errorf("Function.Name = %q, want %q", param.Function.Name, "mytool")
	}
}

func TestToOpenAiToolParam_ParametersType(t *testing.T) {
	def := ToolDefinition{
		Name: "t",
		Desc: "d",
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{},
		},
	}
	param := ToOpenAiToolParam(def)

	fp := param.Function.Parameters
	if fp["type"] != "object" {
		t.Errorf("parameters.type = %v, want %q", fp["type"], "object")
	}
}

func TestToOpenAiToolParam_Properties(t *testing.T) {
	def := ToolDefinition{
		Name: "tool_with_props",
		Desc: "desc",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"path":    {Type: "string", Description: "file path"},
				"content": {Type: "string", Description: "file content"},
			},
			Required: []string{"path", "content"},
		},
	}
	param := ToOpenAiToolParam(def)
	fp := param.Function.Parameters

	props, ok := fp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is not map[string]any, got %T", fp["properties"])
	}
	for _, key := range []string{"path", "content"} {
		if _, exists := props[key]; !exists {
			t.Errorf("expected property %q in converted params", key)
		}
	}
}

func TestToOpenAiToolParam_Required(t *testing.T) {
	def := ToolDefinition{
		Name: "req_tool",
		Desc: "desc",
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{"alpha", "beta"},
		},
	}
	param := ToOpenAiToolParam(def)
	fp := param.Function.Parameters

	required, ok := fp["required"].([]string)
	if !ok {
		t.Fatalf("required is not []string, got %T", fp["required"])
	}
	if len(required) != 2 {
		t.Errorf("required len = %d, want 2", len(required))
	}
}

func TestToOpenAiToolParam_AdditionalPropertiesFalse(t *testing.T) {
	def := ToolDefinition{
		Name: "strict_tool",
		Desc: "desc",
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{},
		},
	}
	param := ToOpenAiToolParam(def)
	fp := param.Function.Parameters

	if fp["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want false", fp["additionalProperties"])
	}
}

func TestToOpenAiToolParam_RealReadTool(t *testing.T) {
	def := (&ReadTool{}).Definition()
	param := ToOpenAiToolParam(def)

	if param.Function.Name != "read" {
		t.Errorf("Function.Name = %q, want %q", param.Function.Name, "read")
	}
	fp := param.Function.Parameters
	props, ok := fp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is not map[string]any")
	}
	if _, exists := props["path"]; !exists {
		t.Error("expected 'path' property in converted ReadTool params")
	}
}

func TestToOpenAiToolParam_RealBashTool(t *testing.T) {
	def := (&BashTool{}).Definition()
	param := ToOpenAiToolParam(def)

	if param.Function.Name != "bash" {
		t.Errorf("Function.Name = %q, want %q", param.Function.Name, "bash")
	}
}
