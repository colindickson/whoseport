package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/bluehoodie/whoseport/internal/display/format"
	displayjson "github.com/bluehoodie/whoseport/internal/display/json"
	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/process"
)

// NOTE: lsof output parsing used to be tested here (TestToProcessInfo), but
// it asserted against a parser input format (a single space-delimited line)
// that no longer exists -- process.NewLsofParser() now parses lsof -F
// field-mode output, and its output no longer resembles this test's
// expectations at all (hence the two failures below). That coverage lives in
// internal/process/parser_test.go (TestLsofParser_Parse) instead, which is
// strictly stronger: it also covers multi-row/multi-PID lsof output and the
// ErrNoService sentinel.
//
// NOTE: FormatBytes, FormatMemory, FormatDuration, VisualWidth,
// StripAnsiCodes, GetStateEmoji, GetStateColor and Truncate are pure
// internal/display/format functions (FormatDuration was formerly duplicated
// in internal/procfs too) that used to have copies of their tests here.
// Those copies exercised nothing in this package and are now covered - at
// least as thoroughly, including the previously-untested maxLen<3 Truncate
// cases and the terminal.Theme-aware GetStateColor path - by
// internal/display/format/formatters_test.go. The few cases that file was
// missing (FormatDuration's "weeks"/"long uptime", several VisualWidth
// cases, and GetStateEmoji/FormatBytes/FormatMemory entirely) were ported
// there before deleting the copies here.

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
