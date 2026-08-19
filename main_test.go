package main

import (
	"context"
	"strings"
	"testing"
)

func Test_executeBashCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
		check   func(t *testing.T, output string)
	}{
		{
			name:    "echo command returns expected output",
			command: "echo Hello, World!",
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, "Hello, World!") {
					t.Errorf("expected output to contain 'Hello, World!', got: %s", output)
				}
			},
		},
		{
			name:    "hostname command returns non-empty output",
			command: "hostname",
			check: func(t *testing.T, output string) {
				if strings.TrimSpace(output) == "" {
					t.Error("expected non-empty hostname, got empty string")
				}
			},
		},
		{
			name:    "invalid command returns error",
			command: "nonexistent_command_xyz123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeBashCommand(context.Background(), tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			t.Logf("executeBashCommand(%q) = %q", tt.command, got)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
