package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bluehoodie/whoseport/internal/display/format"
	displayjson "github.com/bluehoodie/whoseport/internal/display/json"
	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/process"
	"github.com/bluehoodie/whoseport/internal/procfs"
)

// NOTE: lsof output parsing used to be tested here (TestToProcessInfo), but
// it asserted against a parser input format (a single space-delimited line)
// that no longer exists -- process.NewLsofParser() now parses lsof -F
// field-mode output, and its output no longer resembles this test's
// expectations at all (hence the two failures below). That coverage lives in
// internal/process/parser_test.go (TestLsofParser_Parse) instead, which is
// strictly stronger: it also covers multi-row/multi-PID lsof output and the
// ErrNoService sentinel.

// TestFormatBytes tests byte formatting
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
			got := format.FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("format.FormatBytes(%d) = %v, want %v", tt.bytes, got, tt.want)
			}
		})
	}
}

// TestFormatMemory tests memory formatting
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
			got := format.FormatMemory(tt.kb)
			if got != tt.want {
				t.Errorf("format.FormatMemory(%d) = %v, want %v", tt.kb, got, tt.want)
			}
		})
	}
}

// TestFormatDuration tests duration formatting
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
			got := procfs.FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("procfs.FormatDuration(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

// TestVisualWidth tests visual width calculation for emoji and unicode
func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"ascii only", "hello", 5},
		{"single emoji", "🔍", 2},
		{"emoji with text", "🔍 Search", 9},
		{"multiple emoji", "🔍⚙️📊", 6},
		{"mixed", "Hello 🌍 World", 14},
		{"empty string", "", 0},
		{"spaces", "   ", 3},
		{"emoji at end", "Test 🎉", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.VisualWidth(tt.input)
			if got != tt.want {
				t.Errorf("format.VisualWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestStripAnsiCodes tests ANSI code removal
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
			got := format.StripAnsiCodes(tt.input)
			if got != tt.want {
				t.Errorf("format.StripAnsiCodes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGetStateEmoji tests emoji mapping for process states
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
			got := format.GetStateEmoji(tt.state)
			if got != tt.want {
				t.Errorf("format.GetStateEmoji(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestGetStateColor tests color mapping for process states
func TestGetStateColor(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"Running", "\033[32m"},    // Green
		{"Sleeping", "\033[36m"},   // Cyan
		{"Disk Sleep", "\033[37m"}, // White
		{"Zombie", "\033[31m"},     // Red
		{"Stopped", "\033[33m"},    // Yellow
		{"Dead", "\033[37m"},       // White
		{"Unknown", "\033[37m"},    // White
		{"", "\033[37m"},           // White
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := format.GetStateColor(tt.state, nil)
			if got != tt.want {
				t.Errorf("format.GetStateColor(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestTruncate tests string truncation
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"format.Truncate short", "hello world", 5, "he..."},
		{"format.Truncate medium", "hello world", 8, "hello..."},
		{"empty string", "", 5, ""},
		{"very short limit", "hello", 3, "..."},
		// Note: format.Truncate panics when maxLen < 3, which is a bug we'll fix during refactoring
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("format.Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestPrintJSON tests JSON output generation
func TestPrintJSON(t *testing.T) {
	info := &model.ProcessInfo{
		Command: "test-process",
		ID:      12345,
		User:    "testuser",
		FD:      "21u",
		Type:    "IPv4",
		Name:    "*:8080 (LISTEN)",
	}

	// Test displayer output
	var buf bytes.Buffer
	displayer := displayjson.NewDisplayerWithWriter(&buf)
	err := displayer.Display(info)
	if err != nil {
		t.Fatalf("Failed to display JSON: %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var decoded model.ProcessInfo
	output := strings.TrimSpace(buf.String())
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("Produced invalid JSON: %v", err)
	}

	// Verify key fields
	if decoded.Command != "test-process" {
		t.Errorf("JSON Command = %v, want test-process", decoded.Command)
	}
	if decoded.ID != 12345 {
		t.Errorf("JSON ID = %v, want 12345", decoded.ID)
	}
}

// NOTE: createProgressBar and createMemoryBar are now private methods in internal/display/interactive
// These tests are removed since we shouldn't test private implementation details
// The functionality is covered by integration tests that use the full Display() method

// BenchmarkFormatBytes benchmarks the format.FormatBytes function
func BenchmarkFormatBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		format.FormatBytes(1234567890)
	}
}

// BenchmarkVisualWidth benchmarks the format.VisualWidth function
func BenchmarkVisualWidth(b *testing.B) {
	text := "🔍 Hello World 🌍"
	for i := 0; i < b.N; i++ {
		format.VisualWidth(text)
	}
}

// BenchmarkStripAnsiCodes benchmarks the format.StripAnsiCodes function
func BenchmarkStripAnsiCodes(b *testing.B) {
	text := "\033[31m\033[1mHello\033[0m \033[32mWorld\033[0m"
	for i := 0; i < b.N; i++ {
		format.StripAnsiCodes(text)
	}
}

// TestFlagParsing_KillFlag tests that -k flag is parsed correctly
func TestFlagParsing_KillFlag(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	// Test -k short flag
	killFlag = false
	termFlag = false
	flag.BoolVar(&killFlag, "kill", false, "Kill the process using the port")
	flag.BoolVar(&killFlag, "k", false, "Kill the process using the port (shorthand)")
	flag.BoolVar(&termFlag, "term", false, "Terminate the process using the port (SIGTERM)")
	flag.BoolVar(&termFlag, "t", false, "Terminate the process using the port (shorthand)")

	args := []string{"-k", "8080"}
	if err := flag.CommandLine.Parse(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	if !killFlag {
		t.Error("killFlag should be true when -k is passed")
	}
	if termFlag {
		t.Error("termFlag should be false when only -k is passed")
	}
}

// TestFlagParsing_TermFlag tests that -t flag is parsed correctly
func TestFlagParsing_TermFlag(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	// Test -t short flag
	killFlag = false
	termFlag = false
	flag.BoolVar(&killFlag, "kill", false, "Kill the process using the port")
	flag.BoolVar(&killFlag, "k", false, "Kill the process using the port (shorthand)")
	flag.BoolVar(&termFlag, "term", false, "Terminate the process using the port (SIGTERM)")
	flag.BoolVar(&termFlag, "t", false, "Terminate the process using the port (shorthand)")

	args := []string{"-t", "8080"}
	if err := flag.CommandLine.Parse(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	if killFlag {
		t.Error("killFlag should be false when only -t is passed")
	}
	if !termFlag {
		t.Error("termFlag should be true when -t is passed")
	}
}

// TestFlagParsing_LongTermFlag tests that --term flag is parsed correctly
func TestFlagParsing_LongTermFlag(t *testing.T) {
	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	killFlag = false
	termFlag = false
	flag.BoolVar(&killFlag, "kill", false, "Kill the process using the port")
	flag.BoolVar(&killFlag, "k", false, "Kill the process using the port (shorthand)")
	flag.BoolVar(&termFlag, "term", false, "Terminate the process using the port (SIGTERM)")
	flag.BoolVar(&termFlag, "t", false, "Terminate the process using the port (shorthand)")

	args := []string{"--term", "8080"}
	if err := flag.CommandLine.Parse(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	if killFlag {
		t.Error("killFlag should be false when only --term is passed")
	}
	if !termFlag {
		t.Error("termFlag should be true when --term is passed")
	}
}

// TestFlagParsing_MutualExclusion tests that -k and -t cannot be used together
func TestFlagParsing_MutualExclusion(t *testing.T) {
	// This test documents expected behavior when both flags are passed
	// The implementation should handle this gracefully (last flag wins or error)
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	killFlag = false
	termFlag = false
	flag.BoolVar(&killFlag, "kill", false, "Kill the process using the port")
	flag.BoolVar(&killFlag, "k", false, "Kill the process using the port (shorthand)")
	flag.BoolVar(&termFlag, "term", false, "Terminate the process using the port (SIGTERM)")
	flag.BoolVar(&termFlag, "t", false, "Terminate the process using the port (shorthand)")

	args := []string{"-k", "-t", "8080"}
	if err := flag.CommandLine.Parse(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Both flags should be true if both are specified
	// The main.go implementation should validate this and show an error
	if !killFlag {
		t.Error("killFlag should be true when -k is passed")
	}
	if !termFlag {
		t.Error("termFlag should be true when -t is passed")
	}
}

// TestIsNoServiceError tests the isNoServiceError helper function
func TestIsNoServiceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "no service found error",
			err:  process.ErrNoService,
			want: true,
		},
		{
			name: "wrapped no service found error",
			err:  fmt.Errorf("failed to parse lsof output: %w", process.ErrNoService),
			want: true,
		},
		{
			name: "different error",
			err:  errors.New("failed to execute lsof: command not found"),
			want: false,
		},
		{
			name: "empty error message",
			err:  errors.New(""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNoServiceError(tt.err)
			if got != tt.want {
				t.Errorf("isNoServiceError() = %v, want %v", got, tt.want)
			}
		})
	}
}
