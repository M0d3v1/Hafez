package main

import "testing"

func TestRunHelp(t *testing.T) {
	if code := run([]string{"--help"}); code != 0 {
		t.Fatalf("--help = %d, want 0", code)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	tests := [][]string{
		{"--port", "65536"},
		{"--port", "-1"},
		{"--not-a-flag"},
		{"extra"},
		{"--appendfsync", "sometimes"},
	}
	for _, args := range tests {
		if code := run(args); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}
