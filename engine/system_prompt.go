package engine

import (
	"axon/tools"
	"strings"
)

// BuildSystemPrompt constructs a structured system prompt for the agent.
// cwd should be the working directory of the main binary (not the package directory).
func BuildSystemPrompt(container tools.ToolContainer, cwd string) string {
	var b strings.Builder

	b.WriteString("You are Claude Code, a helpful AI coding assistant that operates inside a terminal.\n\n")

	// ── Environment ────────────────────────────────────────────────────────────
	b.WriteString("## Environment\n")
	b.WriteString("- Current working directory: ")
	b.WriteString(cwd)
	b.WriteString("\n\n")

	// ── Available tools ────────────────────────────────────────────────────────
	if len(container.ToolMap) > 0 {
		b.WriteString("## Available Tools\n")
		for _, tool := range container.ToolMap {
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
