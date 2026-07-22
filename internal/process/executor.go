package process

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// LsofExecutor executes lsof to find processes listening on a port.
type LsofExecutor struct{}

// NewLsofExecutor creates a new LsofExecutor.
func NewLsofExecutor() *LsofExecutor {
	return &LsofExecutor{}
}

// Execute runs lsof, filtered to listening TCP sockets on the specified port, and
// returns its field-output (-F) mode output for the Parser to consume.
func (e *LsofExecutor) Execute(port int) ([]byte, error) {
	// -w suppresses warnings about unstattable filesystems (gvfs, network mounts),
	// which lsof emits on stderr while still exiting non-zero.
	cmd := exec.Command("lsof", "-w", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "pcLfatdoPn")

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	return classifyLsofResult(out.Bytes(), stderr.Bytes(), err)
}

// classifyLsofResult decides whether a completed lsof run found no match or hit a
// real error. lsof's exit status alone cannot tell those apart: it exits non-zero
// when nothing matches, when something is genuinely wrong (bad flags, a broken
// wrapper on PATH), and also when it merely emitted a warning while producing
// complete, correct results. So results outrank the exit status, and an empty run
// is distinguished by whether lsof explained itself on stderr.
func classifyLsofResult(stdout, stderr []byte, err error) ([]byte, error) {
	// Finding something beats any exit status: lsof still reports results
	// alongside warnings it exits non-zero for.
	if len(stdout) > 0 {
		return stdout, nil
	}
	if err == nil {
		return nil, nil
	}

	// No results. A genuine failure says why on stderr; a silent non-zero exit
	// means nothing matched, which is not an error.
	if len(stderr) > 0 {
		return nil, fmt.Errorf("lsof failed: %s: %w", bytes.TrimSpace(stderr), err)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, nil
	}
	return nil, fmt.Errorf("failed to run lsof: %w", err)
}
