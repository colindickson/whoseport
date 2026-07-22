package action

import (
	"fmt"
	"os"
	"syscall"
)

// Killer defines the interface for process termination
// Following SOLID Interface Segregation Principle
type Killer interface {
	Kill(pid int, signal syscall.Signal) error
}

// killer implements the Killer interface
type killer struct{}

// NewKiller creates a new Killer implementation
func NewKiller() Killer {
	return &killer{}
}

// Kill terminates a process by PID with the specified signal
// Follows Single Responsibility Principle: only sends signals to processes
func (k *killer) Kill(pid int, signal syscall.Signal) error {
	// os.FindProcess never fails on Unix; the real error surfaces from Signal below.
	process, _ := os.FindProcess(pid)

	if err := process.Signal(signal); err != nil {
		return fmt.Errorf("process with PID %d does not exist or is not accessible: %w", pid, err)
	}

	return nil
}
