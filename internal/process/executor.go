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
	cmd := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-F", "pcLftdoPn")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// lsof exits non-zero when nothing matches the selection criteria;
			// that just means no process is listening on this port.
			return out.Bytes(), nil
		}
		return nil, fmt.Errorf("failed to run lsof: %w", err)
	}

	return out.Bytes(), nil
}
