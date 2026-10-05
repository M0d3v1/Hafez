package store

import "testing"

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "", true},
		{"*", "abc", true},
		{"h?llo", "hello", true},
		{"h?llo", "hllo", false},
		{"h*llo", "hllo", true},
		{"h*llo", "heeeello", true},
		{"h[ae]llo", "hello", true},
		{"h[ae]llo", "hallo", true},
		{"h[ae]llo", "hillo", false},
		{"h[^e]llo", "hallo", true},
		{"h[^e]llo", "hello", false},
		{"h[a-c]llo", "hbllo", true},
		{"h[a-c]llo", "hello", false},
		{`\*`, "*", true},
		{`\*`, "abc", false},
		{`a\*`, "a*", true},
		{"foo.bar", "foo.bar", true},
		{"foo.bar", "fooXbar", false},
		{"[", "[", true},
		{"[", "a", false},
		{"[]a]", "]", true},
		{"a-", "a-", true},
		{"", "", true},
		{"", "a", false},
		{"?", "", false},
		{`\`, `\`, true},
	}
	for _, tt := range tests {
		if got := globMatch(tt.pattern, tt.s); got != tt.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
		}
	}
}
