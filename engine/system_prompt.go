package engine

import (
	"axon/tools"
	"strings"
)

// SystemPromptOptions holds all inputs needed to construct the system prompt.
// Using an options struct makes future additions (CustomPrompt, ContextFiles, …)
// backward-compatible — callers that already set the fields they care about
// are unaffected when new optional fields are added.
type SystemPromptOptions struct {
	Cwd       string
	Container *tools.ToolContainer
}

// BuildSystemPrompt constructs a structured system prompt for the agent.
func BuildSystemPrompt(opts SystemPromptOptions) string {
	var b strings.Builder

	b.WriteString("You are axon, a helpful AI coding assistant that create by zane0x.you operates inside a terminal.\n\n")

	// ── Environment ────────────────────────────────────────────────────────────
	b.WriteString("## Environment\n")
	b.WriteString("- Current working directory: ")
	b.WriteString(opts.Cwd)
	b.WriteString("\n\n")

	// ── Available tools ────────────────────────────────────────────────────────
	if opts.Container != nil && len(opts.Container.ToolMap) > 0 {
		b.WriteString("## Available Tools\n")
		for _, tool := range opts.Container.ToolMap {
			def := tool.Definition()
			b.WriteString("### ")
			b.WriteString(def.Name)
			b.WriteString("\n")
			b.WriteString(def.Desc)
			b.WriteString("\n")
			if len(def.Guidelines) > 0 {
				b.WriteString("\n**Guidelines:**\n")
				for _, g := range def.Guidelines {
					b.WriteString("- ")
					b.WriteString(g)
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
	}

	// ── General guidelines ─────────────────────────────────────────────────────
	b.WriteString("## General Guidelines\n")
	b.WriteString("- Think step-by-step before acting; prefer targeted, reversible changes.\n")
	b.WriteString("- Always read a file before editing it to ensure old_text matches exactly.\n")
	b.WriteString("- When output may be large, pipe through head/tail/grep to limit results.\n")
	b.WriteString("- Confirm with the user before performing destructive or irreversible operations.\n")

	return b.String()
}
