package process

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestClassifyLsofResult exercises the exit-status classification logic in
// isolation (Fix 2): a non-zero exit is "no match" only when lsof produced no
// output at all on stdout or stderr; any output on either stream means lsof hit
// a real problem (bad flags, broken wrapper, unsupported option), and that must
// surface as an error - not be swallowed as "nothing is listening".
func TestClassifyLsofResult(t *testing.T) {
	exitErr := &exec.ExitError{}

	tests := []struct {
		name       string
		stdout     []byte
		stderr     []byte
		err        error
		wantErr    bool
		wantOutput []byte
	}{
		{
			name:       "success",
			stdout:     []byte("p123\n"),
			err:        nil,
			wantOutput: []byte("p123\n"),
		},
		{
			name:       "non-zero exit, no output at all: no match",
			stdout:     nil,
			stderr:     nil,
			err:        exitErr,
			wantOutput: nil,
		},
		{
			name:    "non-zero exit with stderr text: real error, not swallowed",
			stdout:  nil,
			stderr:  []byte("lsof: unsupported option: -sTCP:LISTEN\n"),
			err:     exitErr,
			wantErr: true,
		},
		{
			// lsof exits non-zero for a warning even when it found the port's
			// listener. Discarding those results reports a false negative on any
			// host with an unstattable mount (gvfs, SMB), which is the worst
			// possible answer: confidently wrong.
			name:       "warning on stderr alongside results: results win",
			stdout:     []byte("p123\ncpython3.11\n"),
			stderr:     []byte("lsof: WARNING: can't stat() the smbfs file system /Volumes/x\n"),
			err:        exitErr,
			wantOutput: []byte("p123\ncpython3.11\n"),
		},
		{
			name:    "non-error exec failure (e.g. lsof not on PATH)",
			stdout:  nil,
			stderr:  nil,
			err:     errors.New("exec: \"lsof\": executable file not found in $PATH"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := classifyLsofResult(tt.stdout, tt.stderr, tt.err)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("classifyLsofResult() error = nil, want error")
				}
				if len(tt.stderr) > 0 && !strings.Contains(err.Error(), strings.TrimSpace(string(tt.stderr))) {
					t.Errorf("classifyLsofResult() error = %q, want it to contain stderr text %q", err, tt.stderr)
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyLsofResult() unexpected error: %v", err)
			}
			if string(out) != string(tt.wantOutput) {
				t.Errorf("classifyLsofResult() output = %q, want %q", out, tt.wantOutput)
			}
		})
	}
}

// TestLsofExecutor_Execute exercises the real lsof binary (no mocking): it is the
// simplest way to verify the grep subprocess is gone and that lsof's "nothing
// matched" exit status is not surfaced as an error (Fix 2/3).
func TestLsofExecutor_Execute(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available on this machine")
	}

	t.Run("port with a real listener", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to open listener: %v", err)
		}
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port

		out, err := NewLsofExecutor().Execute(port)
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		want := fmt.Sprintf("p%d", os.Getpid())
		if !strings.Contains(string(out), want) {
			t.Errorf("Execute() output = %q, want it to contain %q (this process's PID field)", out, want)
		}
		// Fix 2: a `grep LISTEN` subprocess would also match a process whose own
		// name literally contains "LISTEN"; verify we get -F output instead of
		// plain grep-filtered text.
		if !strings.Contains(string(out), "\nf") {
			t.Errorf("Execute() output = %q, does not look like lsof -F field output", out)
		}
	})

	t.Run("port with no listener", func(t *testing.T) {
		// Bind then close immediately to get a port very unlikely to have a
		// listener at the moment we query it.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to open listener: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()

		out, err := NewLsofExecutor().Execute(port)
		if err != nil {
			t.Fatalf("Execute() unexpected error for unmatched port: %v", err)
		}
		if strings.Contains(string(out), "p"+strconv.Itoa(os.Getpid())) {
			t.Errorf("Execute() unexpectedly found a listener on freshly-closed port %d", port)
		}
	})
}
