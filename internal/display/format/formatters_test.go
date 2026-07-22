package format

import (
	"testing"
	"time"

	"github.com/bluehoodie/whoseport/internal/terminal"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"bytes", 500, "500 B"},
		{"kilobytes", 1024, "1.00 KB"},
		{"megabytes", 1024 * 1024, "1.00 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.00 GB"},
		{"mixed KB", 1536, "1.50 KB"},
		{"mixed MB", 1024*1024 + 512*1024, "1.50 MB"},
		{"large number", 9876543210, "9.20 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %v, want %v", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		name string
		kb   int64
		want string
	}{
		{"zero", 0, "0 KB"},
		{"kilobytes", 100, "100 KB"},
		{"megabytes", 1024, "1.00 MB"},
		{"gigabytes", 1024 * 1024, "1.00 GB"},
		{"mixed MB", 1536, "1.50 MB"},
		{"large GB", 2560 * 1024, "2.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMemory(tt.kb)
			if got != tt.want {
				t.Errorf("FormatMemory(%d) = %v, want %v", tt.kb, got, tt.want)
			}
		})
	}
}

func TestGetStateEmoji(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"Running", "🟢"},
		{"Sleeping", "🔵"},
		{"Sleeping (interruptible)", "🔵"},
		{"Waiting (uninterruptible)", "⏸"},
		{"Zombie", "💀"},
		{"Stopped", "🟡"},
		{"Dead", "⚪"},
		{"Unknown", "⚪"},
		{"", "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := GetStateEmoji(tt.state)
			if got != tt.want {
				t.Errorf("GetStateEmoji(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0s"},
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m 0s"},
		{"hours", 2 * time.Hour, "2h 0m 0s"},
		{"days", 24 * time.Hour, "1d 0h 0m 0s"},
		{"weeks", 7 * 24 * time.Hour, "7d 0h 0m 0s"},
		{"mixed", 2*time.Hour + 30*time.Minute + 15*time.Second, "2h 30m 15s"},
		{"long uptime", 10*24*time.Hour + 5*time.Hour + 23*time.Minute, "10d 5h 23m 0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

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
		{"emoji with text", "🔍 Search", 9},
		{"mixed", "Hello 🌍 World", 14},
		{"empty string", "", 0},
		{"spaces", "   ", 3},
		{"emoji at end", "Test 🎉", 7},
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
