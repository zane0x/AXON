package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// ─── stub tool ───────────────────────────────────────────────────────────────

// stubTool is a minimal AgentTool implementation used only for container tests.
type stubTool struct {
	name string
}

func (s *stubTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: s.name,
		Desc: "stub tool for testing",
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{},
		},
	}
}

func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (ToolResult, error) {
	return ToolResult{Content: "stub:" + s.name}, nil
}

// ─── NewToolContainer ─────────────────────────────────────────────────────────

func TestNewToolContainer_NotNil(t *testing.T) {
	c := NewToolContainer()
	if c == nil {
		t.Fatal("NewToolContainer() returned nil")
	}
}

func TestNewToolContainer_EmptyMap(t *testing.T) {
	c := NewToolContainer()
	if len(c.ToolMap) != 0 {
		t.Errorf("expected empty ToolMap, got %d entries", len(c.ToolMap))
	}
}

// ─── ToolContainer.RegisterTool ───────────────────────────────────────────────

func TestRegisterTool_SingleTool(t *testing.T) {
	c := NewToolContainer()
	c.RegisterTool(&stubTool{name: "alpha"})
	if _, ok := c.ToolMap["alpha"]; !ok {
		t.Error("expected 'alpha' in ToolMap after RegisterTool")
	}
}

func TestRegisterTool_MultipleTools(t *testing.T) {
	c := NewToolContainer()
	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		c.RegisterTool(&stubTool{name: n})
	}
	if len(c.ToolMap) != len(names) {
		t.Errorf("ToolMap len = %d, want %d", len(c.ToolMap), len(names))
	}
	for _, n := range names {
		if _, ok := c.ToolMap[n]; !ok {
			t.Errorf("expected %q in ToolMap", n)
		}
	}
}

func TestRegisterTool_OverwritesSameName(t *testing.T) {
	c := NewToolContainer()
	c.RegisterTool(&stubTool{name: "dup"})
	c.RegisterTool(&stubTool{name: "dup"})
	if len(c.ToolMap) != 1 {
		t.Errorf("expected 1 entry after registering same name twice, got %d", len(c.ToolMap))
	}
}

func TestRegisterTool_RealTools(t *testing.T) {
	c := NewToolContainer()
	c.RegisterTool(&ReadTool{})
	c.RegisterTool(&WriteTool{})
	c.RegisterTool(&EditTool{})
	c.RegisterTool(&BashTool{})

	for _, name := range []string{"read", "write", "edit", "bash"} {
		if _, ok := c.ToolMap[name]; !ok {
			t.Errorf("expected %q in ToolMap", name)
		}
	}
}

// ─── ToolDefinition field coverage ───────────────────────────────────────────

func TestToolDefinition_Fields(t *testing.T) {
	def := ToolDefinition{
		Name:    "mytool",
		Desc:    "my desc",
		Snippet: "my snippet",
		Guidelines: []string{
			"guideline one",
			"guideline two",
		},
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"key": {Type: "string", Description: "a key"},
			},
			Required: []string{"key"},
		},
	}

	if def.Name != "mytool" {
		t.Errorf("Name = %q, want %q", def.Name, "mytool")
	}
	if def.Snippet != "my snippet" {
		t.Errorf("Snippet = %q, want %q", def.Snippet, "my snippet")
	}
	if len(def.Guidelines) != 2 {
		t.Errorf("Guidelines len = %d, want 2", len(def.Guidelines))
	}
	if _, ok := def.Parameters.Properties["key"]; !ok {
		t.Error("expected 'key' in Properties")
	}
}

// ─── ToolResult ───────────────────────────────────────────────────────────────

func TestToolResult_Content(t *testing.T) {
	r := ToolResult{Content: "some output"}
	if r.Content != "some output" {
		t.Errorf("Content = %q, want %q", r.Content, "some output")
	}
}
