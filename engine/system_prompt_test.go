package engine

import (
	"axon/tools"
	"fmt"
	"testing"
)

func Test_buildSystemPrompt(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		container tools.ToolContainer
		want      string
	}{
		{ // TODO: Add test cases.
			name:      "case1",
			container: *tools.NewToolContainer(),
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSystemPrompt(tt.container, "/tmp/test-cwd")
			fmt.Println(got)
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("buildSystemPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
