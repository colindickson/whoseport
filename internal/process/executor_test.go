package process

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

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
