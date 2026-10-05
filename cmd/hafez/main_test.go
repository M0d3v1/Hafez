package main

import "testing"

func TestRunRejectsBadFlags(t *testing.T) {
	tests := [][]string{
		{"--port", "65536"},
		{"--port", "-1"},
		{"--not-a-flag"},
		{"extra"},
	}
	for _, args := range tests {
		if code := run(args); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}
