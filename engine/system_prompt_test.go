package engine

import (
	"axon/tools"
	"strings"
	"testing"
)

// newContainerWith registers the given tools and returns the container.
func newContainerWith(ts ...tools.AgentTool) tools.ToolContainer {
	c := tools.NewToolContainer()
	for _, t := range ts {
		c.RegisterTool(t)
	}
	return *c
}

// ─── BuildSystemPrompt ────────────────────────────────────────────────────────

func TestBuildSystemPrompt_ContainsHeader(t *testing.T) {
	got := BuildSystemPrompt(*tools.NewToolContainer(), "/tmp/test")
	if !strings.Contains(got, "You are Claude Code") {
		t.Errorf("expected header, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_ContainsCWD(t *testing.T) {
	cwd := "/home/user/project"
	got := BuildSystemPrompt(*tools.NewToolContainer(), cwd)
	if !strings.Contains(got, cwd) {
		t.Errorf("expected cwd %q in prompt, got:\n%s", cwd, got)
	}
}

func TestBuildSystemPrompt_EmptyCWD(t *testing.T) {
	got := BuildSystemPrompt(*tools.NewToolContainer(), "")
	// Should still produce a valid prompt without panicking.
	if !strings.Contains(got, "## Environment") {
		t.Errorf("expected Environment section, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_NoTools_NoToolsSection(t *testing.T) {
	got := BuildSystemPrompt(*tools.NewToolContainer(), "/tmp")
	if strings.Contains(got, "## Available Tools") {
		t.Errorf("expected no Available Tools section when container is empty, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_WithTools_ContainsToolName(t *testing.T) {
	c := newContainerWith(&tools.ReadTool{}, &tools.WriteTool{}, &tools.EditTool{}, &tools.BashTool{})
	got := BuildSystemPrompt(c, "/tmp")

	if !strings.Contains(got, "## Available Tools") {
		t.Errorf("expected Available Tools section, got:\n%s", got)
	}
	for _, name := range []string{"read", "write", "edit", "bash"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected tool %q in prompt", name)
		}
	}
}

func TestBuildSystemPrompt_WithTools_ContainsGuidelines(t *testing.T) {
	c := newContainerWith(&tools.ReadTool{})
	got := BuildSystemPrompt(c, "/tmp")
	if !strings.Contains(got, "**Guidelines:**") {
		t.Errorf("expected Guidelines block, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_ContainsGeneralGuidelines(t *testing.T) {
	got := BuildSystemPrompt(*tools.NewToolContainer(), "/tmp")
	if !strings.Contains(got, "## General Guidelines") {
		t.Errorf("expected General Guidelines section, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_SingleTool_ContainsDesc(t *testing.T) {
	c := newContainerWith(&tools.BashTool{})
	got := BuildSystemPrompt(c, "/tmp")
	desc := "Execute a bash command"
	if !strings.Contains(got, desc) {
		t.Errorf("expected tool desc %q in prompt, got:\n%s", desc, got)
	}
}
