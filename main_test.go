package main

import "testing"

func Test_executeBashCommand(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		command string
		want    string
		wantErr bool
	}{
		{"Test valid command", "echo Hello, World!", "Hello, World!\n", false},
		{"Test valid command", "hostname -i", "192.168.16.2\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := executeBashCommand(tt.command)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("executeBashCommand() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("executeBashCommand() succeeded unexpectedly")
			}
			t.Logf("executeBashCommand() = %v, want %v", got, tt.want)
			if got != tt.want {
				t.Errorf("executeBashCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
