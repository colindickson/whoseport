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
	cmd := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "pcLfatdoPn")

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	return classifyLsofResult(out.Bytes(), stderr.Bytes(), err)
}

// classifyLsofResult decides whether a completed lsof run found no match or hit a
// real error. lsof exits non-zero both when nothing matches the selection criteria
// and when something is actually wrong (bad flags, a broken wrapper on PATH, an
// lsof build that rejects one of our options); the only way to tell those apart is
// that a genuine problem produces output on stdout and/or stderr, while "no match"
// produces neither. Treating every non-zero exit as "no match" would silently
// report a false negative, which is worse than surfacing an error.
func classifyLsofResult(stdout, stderr []byte, err error) ([]byte, error) {
	if err == nil {
		return stdout, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(stdout) == 0 && len(stderr) == 0 {
		return nil, nil
	}

	if len(stderr) > 0 {
		return nil, fmt.Errorf("lsof failed: %s: %w", bytes.TrimSpace(stderr), err)
	}
	return nil, fmt.Errorf("failed to run lsof: %w", err)
}
