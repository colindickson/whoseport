package format

import (
	"testing"

	"github.com/bluehoodie/whoseport/internal/terminal"
)

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

func TestStripAnsiCodes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no codes", "hello", "hello"},
		{"simple color", "\033[31mred\033[0m", "red"},
		{"multiple codes", "\033[1m\033[31mbold red\033[0m", "bold red"},
		{"256 color", "\033[38;5;208morange\033[0m", "orange"},
		{"mixed", "Hello \033[31mRed\033[0m World", "Hello Red World"},
		{"empty", "", ""},
		{"only codes", "\033[31m\033[0m", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAnsiCodes(tt.input)
			if got != tt.want {
				t.Errorf("StripAnsiCodes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetStateColor(t *testing.T) {
	// nil falls back to terminal.DefaultTheme(), byte-identical to the
	// old hard-coded colors.
	nilCases := []struct {
		state string
		want  string
	}{
		{"Running", "\033[32m"},
		{"Sleeping", "\033[36m"},
		{"Zombie", "\033[31m"},
		{"Stopped", "\033[33m"},
		{"Unknown", "\033[37m"},
	}
	for _, tt := range nilCases {
		t.Run("nil/"+tt.state, func(t *testing.T) {
			got := GetStateColor(tt.state, nil)
			if got != tt.want {
				t.Errorf("GetStateColor(%q, nil) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}

	// A real theme must actually be read, not ignored.
	custom := terminal.Theme{
		StateRunning:  "\033[95m",
		StateSleeping: "\033[96m",
		StateZombie:   "\033[97m",
		StateStopped:  "\033[93m",
		StateDefault:  "\033[90m",
	}
	themeCases := []struct {
		state string
		want  string
	}{
		{"Running", custom.StateRunning},
		{"Sleeping", custom.StateSleeping},
		{"Zombie", custom.StateZombie},
		{"Stopped", custom.StateStopped},
		{"Unknown", custom.StateDefault},
	}
	for _, tt := range themeCases {
		t.Run("theme/"+tt.state, func(t *testing.T) {
			got := GetStateColor(tt.state, custom)
			if got != tt.want {
				t.Errorf("GetStateColor(%q, custom) = %q, want %q", tt.state, got, tt.want)
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
