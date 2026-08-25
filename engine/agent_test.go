package engine

import (
	"testing"
)

func TestFormatToolResultForCLI(t *testing.T) {
	tests := []struct {
		toolName string
		content  string
		expected string
	}{
		{
			toolName: "read",
			content:  "some file contents",
			expected: "(Read 18 bytes)",
		},
		{
			toolName: "read",
			content:  "",
			expected: "(Read 0 bytes)",
		},
		{
			toolName: "edit",
			content:  "Successfully edited auth/validate.go",
			expected: "Successfully edited auth/validate.go",
		},
		{
			toolName: "bash",
			content:  "  PASS ok auth/validation [0.12s]\n\n",
			expected: "PASS ok auth/validation [0.12s]",
		},
	}

	for _, tt := range tests {
		got := formatToolResultForCLI(tt.toolName, tt.content)
		if got != tt.expected {
			t.Errorf("formatToolResultForCLI(%q, %q) = %q; expected %q", tt.toolName, tt.content, got, tt.expected)
		}
	}
}
