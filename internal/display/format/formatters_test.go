package format

import "testing"

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"ascii", "hello", 5},
		{"emoji", "🔍", 2},
		{"emoji variation selector", "⚙️  PROCESS IDENTITY", 20},
		{"multiple emoji", "🔍⚙️📊", 6},
		{"zwj sequence", "👩‍💻", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisualWidth(tt.input)
			if got != tt.want {
				t.Fatalf("VisualWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		// Preserve the pre-existing behaviour asserted by cmd/whoseport/main_test.go.
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"short", "hello world", 5, "he..."},
		{"medium", "hello world", 8, "hello..."},
		{"empty string", "", 5, ""},
		{"very short limit", "hello", 3, "..."},
		// maxLen < 3 must never panic (this was the bug).
		{"maxLen 2", "hello", 2, ".."},
		{"maxLen 1", "hello", 1, "."},
		{"maxLen 0", "hello", 0, ""},
		{"maxLen negative", "hello", -5, ""},
		{"maxLen 2 fits", "ab", 2, "ab"},
		// Multi-byte / emoji input must not split mid-rune.
		{"multibyte fits", "héllo", 10, "héllo"},
		{"multibyte truncated", "héllo wörld", 5, "hé..."},
		{"emoji truncated", "🔍🔍🔍🔍🔍🔍", 5, "🔍..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if !isValidUTF8(got) {
				t.Errorf("Truncate(%q, %d) = %q is not valid UTF-8", tt.input, tt.maxLen, got)
			}
		})
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
