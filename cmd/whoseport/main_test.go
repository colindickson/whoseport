package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluehoodie/whoseport/internal/display/format"
	displayjson "github.com/bluehoodie/whoseport/internal/display/json"
	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/process"
)

// binPath is the whoseport binary built once by TestMain and exercised as a
// subprocess by the TestCLI_* tests below.
var binPath string

// TestMain builds the real whoseport binary once so the CLI tests can run it
// as a subprocess and assert on its actual exit code/stdout/stderr, rather
// than reimplementing flag-parsing logic that has nothing to do with
// whoseport's own validation.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "whoseport-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: MkdirTemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "whoseport")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// runCLI runs the built whoseport binary with the given args and returns its
// stdout, stderr, and exit code. Stdin is left unset, which os/exec connects
// to the null device, so a test can never accidentally block on an
// interactive prompt.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
	}
	t.Fatalf("failed to run whoseport: %v", err)
	return "", "", -1
}

// freeTCPPort returns a port with nothing listening on it, by opening then
// immediately closing a listener the kernel assigned. There is a small,
// unavoidable race if something else grabs the port before the CLI runs it,
// but in practice this is reliable enough for a test.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestCLI_MutualExclusion verifies that combining -k and -t is rejected with
// a clear error rather than silently picking one signal over the other -
// main.go's actual validation (main.go:69-73), which the flag package itself
// has no opinion on. This never touches a real process: main.go validates
// the flags before it ever looks up a port.
func TestCLI_MutualExclusion(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "-k", "-t", "8080")

	if exitCode == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "cannot use both") || !strings.Contains(combined, "-k") || !strings.Contains(combined, "-t") {
		t.Errorf("output = %q, want a message mentioning both -k and -t cannot be combined", combined)
	}
}

// TestCLI_MissingPort verifies that running with no port argument fails with
// a message the user can act on, rather than a stdlib flag-parsing panic or
// silent default.
func TestCLI_MissingPort(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t)

	if exitCode == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	if !strings.Contains(stdout+stderr, "missing port number") {
		t.Errorf("output = %q, want it to mention a missing port number", stdout+stderr)
	}
}

// TestCLI_NonIntegerPort verifies a non-numeric port argument is rejected
// with a message identifying the actual problem, instead of a generic
// strconv error leaking through.
func TestCLI_NonIntegerPort(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "abc")

	if exitCode == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	if !strings.Contains(stdout+stderr, "port must be an integer") {
		t.Errorf("output = %q, want it to say the port must be an integer", stdout+stderr)
	}
}

// TestCLI_JSONNoServiceIsCleanOnStdout verifies that --json against a port
// nobody is listening on fails with the diagnostic on stderr only, keeping
// stdout parseable as JSON (or empty) for scripting - the whole reason
// --json implies -n (main.go:64-66).
func TestCLI_JSONNoServiceIsCleanOnStdout(t *testing.T) {
	port := freeTCPPort(t)
	stdout, stderr, exitCode := runCLI(t, "--json", fmt.Sprintf("%d", port))

	if exitCode == 0 {
		t.Errorf("exit code = 0, want non-zero (nothing listening on port %d)", port)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty so --json output stays parseable", stdout)
	}
	if !strings.Contains(stderr, fmt.Sprintf("port %d", port)) {
		t.Errorf("stderr = %q, want it to mention port %d", stderr, port)
	}
}

// TestCLI_Help verifies --help documents every flag whoseport actually
// registers, so a change to main.go's flag set can't silently drop one from
// the usage text.
func TestCLI_Help(t *testing.T) {
	stdout, _, _ := runCLI(t, "--help")

	for _, want := range []string{"-k", "--kill", "-t", "--term", "-n", "--no-interactive", "--json"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--help output missing %q\noutput:\n%s", want, stdout)
		}
	}
}

// NOTE: lsof output parsing used to be tested here (TestToProcessInfo), but
// it asserted against a parser input format (a single space-delimited line)
// that no longer exists -- process.NewLsofParser() now parses lsof -F
// field-mode output, so the old test had been failing outright since the
// parser was rewritten. That coverage lives in
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
