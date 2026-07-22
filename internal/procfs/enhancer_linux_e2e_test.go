//go:build linux

package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/testutil"
)

// writeFile is a small helper to reduce repetition when building the fake
// proc tree below.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestEnhanceEndToEnd builds a fake /proc tree from the shared testutil
// fixtures and runs the real Enhance() pipeline against it, verifying that
// every enhancer stage (status, stat, io, limits, fd/environ counts,
// children, parent command, and both IPv4/IPv6 network connections) wires
// its output into the right ProcessInfo field. This is the test that would
// have caught A2/A3's parsing bugs end-to-end rather than function-by-function.
func TestEnhanceEndToEnd(t *testing.T) {
	root := t.TempDir()
	const pid = 12345
	const ppid = 1000
	pidDir := filepath.Join(root, fmt.Sprintf("%d", pid))

	writeFile(t, filepath.Join(pidDir, "cmdline"), "node\x00server.js")
	writeFile(t, filepath.Join(pidDir, "status"), testutil.SampleProcStatus())
	writeFile(t, filepath.Join(pidDir, "stat"), testutil.SampleProcStat())
	writeFile(t, filepath.Join(pidDir, "io"), testutil.SampleProcIO())
	writeFile(t, filepath.Join(pidDir, "limits"), testutil.SampleProcLimits())
	writeFile(t, filepath.Join(pidDir, "environ"), "VAR1=a\x00VAR2=b\x00")
	writeFile(t, filepath.Join(root, "net", "tcp"), testutil.SampleProcNetTCP())
	writeFile(t, filepath.Join(root, "net", "tcp6"), testutil.SampleProcNetTCP6())
	writeFile(t, filepath.Join(root, "net", "udp"), testutil.SampleProcNetUDP())
	writeFile(t, filepath.Join(root, "net", "udp6"), "")
	writeFile(t, filepath.Join(root, "stat"), testutil.SampleProcBootStat())

	// Parent process: only cmdline is needed to resolve ParentCommand.
	writeFile(t, filepath.Join(root, fmt.Sprintf("%d", ppid), "cmdline"), "/usr/bin/bash\x00")

	// Working dir / exe symlinks.
	if err := os.Symlink("/app", filepath.Join(pidDir, "cwd")); err != nil {
		t.Fatal(err)
	}
	exeTarget := filepath.Join(root, "fake-node-binary")
	writeFile(t, exeTarget, "binary-contents")
	if err := os.Symlink(exeTarget, filepath.Join(pidDir, "exe")); err != nil {
		t.Fatal(err)
	}

	// fd directory: three sockets matching the net/tcp and net/udp fixture
	// inodes above, one socket matching the tcp6 fixture, and one plain fd
	// that must be counted in OpenFDs but must NOT show up as a connection.
	fdDir := filepath.Join(pidDir, "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fdLinks := map[string]string{
		"0": "socket:[123456]", // net/tcp line 1 (0.0.0.0:8080 LISTEN)
		"1": "socket:[789012]", // net/tcp line 2 (127.0.0.1:3306 LISTEN)
		"2": "socket:[456789]", // net/udp line 1 (0.0.0.0:5000)
		"3": "socket:[999999]", // net/tcp6 line 1 (::1:9090 LISTEN)
		"4": "/dev/null",       // not a socket at all
	}
	for fd, target := range fdLinks {
		if err := os.Symlink(target, filepath.Join(fdDir, fd)); err != nil {
			t.Fatal(err)
		}
	}

	// Task/children: two children of the process.
	writeFile(t, filepath.Join(pidDir, "task", "12345", "children"), "23456 23457\n")

	enhancer := &ProcessEnhancer{root: root}
	info := &model.ProcessInfo{ID: pid, Command: "node"}

	if err := enhancer.Enhance(info); err != nil {
		t.Fatalf("Enhance() error = %v", err)
	}

	if info.FullCommand != "node server.js" {
		t.Errorf("FullCommand = %q, want %q", info.FullCommand, "node server.js")
	}
	if info.PPid != ppid {
		t.Errorf("PPid = %d, want %d", info.PPid, ppid)
	}
	if info.ParentCommand != "bash" {
		t.Errorf("ParentCommand = %q, want %q", info.ParentCommand, "bash")
	}
	if info.Threads != 4 {
		t.Errorf("Threads = %d, want 4", info.Threads)
	}
	if info.MemoryRSS != 48000 {
		t.Errorf("MemoryRSS = %d, want 48000", info.MemoryRSS)
	}
	if info.WorkingDir != "/app" {
		t.Errorf("WorkingDir = %q, want /app", info.WorkingDir)
	}
	if info.ExePath != exeTarget {
		t.Errorf("ExePath = %q, want %q", info.ExePath, exeTarget)
	}
	if info.ExeSize != int64(len("binary-contents")) {
		t.Errorf("ExeSize = %d, want %d", info.ExeSize, len("binary-contents"))
	}
	if info.OpenFDs != len(fdLinks) {
		t.Errorf("OpenFDs = %d, want %d", info.OpenFDs, len(fdLinks))
	}
	if info.EnvCount != 2 {
		t.Errorf("EnvCount = %d, want 2", info.EnvCount)
	}
	if info.ChildCount != 2 {
		t.Errorf("ChildCount = %d, want 2", info.ChildCount)
	}
	if info.IOReadBytes != 1048576 {
		t.Errorf("IOReadBytes = %d, want 1048576", info.IOReadBytes)
	}
	if info.MaxFDs != 1024 {
		t.Errorf("MaxFDs = %d, want 1024", info.MaxFDs)
	}
	if info.MemoryLimit != -1 {
		t.Errorf("MemoryLimit = %d, want -1 (unlimited)", info.MemoryLimit)
	}

	if len(info.TCPConns) != 3 {
		t.Fatalf("TCPConns = %v, want 3 entries (2 IPv4 + 1 IPv6)", info.TCPConns)
	}
	if len(info.UDPConns) != 1 {
		t.Fatalf("UDPConns = %v, want 1 entry", info.UDPConns)
	}
	if info.UDPConns[0] != "*:5000 -> *:0" {
		t.Errorf("UDPConns[0] = %q, want %q", info.UDPConns[0], "*:5000 -> *:0")
	}
	if info.NetworkConns != 4 {
		t.Errorf("NetworkConns = %d, want 4", info.NetworkConns)
	}

	foundIPv6 := false
	for _, c := range info.TCPConns {
		if c == "::1:9090 -> *:0 [LISTEN]" {
			foundIPv6 = true
		}
	}
	if !foundIPv6 {
		t.Errorf("TCPConns = %v, want an ::1:9090 -> *:0 [LISTEN] entry read from net/tcp6", info.TCPConns)
	}
}
