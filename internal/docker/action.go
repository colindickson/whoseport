package docker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/bluehoodie/whoseport/internal/terminal"
)

// Action represents a Docker container action.
type Action int

const (
	ActionCancel Action = iota
	ActionStop
	ActionStopAndRemove
	ActionRemove
)

// ActionHandler handles Docker container actions.
type ActionHandler struct {
	reader io.Reader
	writer io.Writer
}

// NewActionHandler creates a new Docker action handler.
//
// writer is os.Stderr, not os.Stdout: action progress/result messages
// (and, since PromptAction shares the same writer, the interactive
// Docker action prompt) are diagnostics, not the requested output, and
// must never land in a stdout stream a caller may be parsing (e.g. --json).
func NewActionHandler() *ActionHandler {
	return &ActionHandler{reader: os.Stdin, writer: os.Stderr}
}

// NewActionHandlerWithIO creates a new action handler with custom reader/writer (for testing).
func NewActionHandlerWithIO(reader io.Reader, writer io.Writer) *ActionHandler {
	return &ActionHandler{reader: reader, writer: writer}
}

// PromptAction prompts the user to select an action for the Docker container.
func (h *ActionHandler) PromptAction(info *ContainerInfo) Action {
	fmt.Fprintf(h.writer, "%s%s🐳 Container %s (%s) - Select action:%s\n",
		terminal.ColorBold, terminal.ColorYellow, info.Name, info.ShortID, terminal.ColorReset)
	fmt.Fprintf(h.writer, "  [1] Stop container (docker stop)\n")
	fmt.Fprintf(h.writer, "  [2] Stop and remove container (docker stop + docker rm)\n")
	fmt.Fprintf(h.writer, "  [3] Force remove running container (docker rm -f)\n")
	fmt.Fprintf(h.writer, "  [4] Cancel\n")
	fmt.Fprintf(h.writer, "%sChoice [4]:%s ", terminal.ColorBold, terminal.ColorReset)

	scanner := bufio.NewScanner(h.reader)

	for {
		if !scanner.Scan() {
			return ActionCancel
		}

		choice := strings.TrimSpace(scanner.Text())

		// Default to Cancel if empty
		if choice == "" {
			choice = "4"
		}

		switch choice {
		case "1":
			return ActionStop
		case "2":
			return ActionStopAndRemove
		case "3":
			return ActionRemove
		case "4":
			return ActionCancel
		default:
			fmt.Fprintf(h.writer, "%sInvalid choice. Please enter 1-4.%s\n", terminal.ColorYellow, terminal.ColorReset)
			fmt.Fprintf(h.writer, "%sChoice [4]:%s ", terminal.ColorBold, terminal.ColorReset)
		}
	}
}

// ExecuteAction executes the selected Docker action.
func (h *ActionHandler) ExecuteAction(action Action, info *ContainerInfo) error {
	switch action {
	case ActionStop:
		return h.stopContainer(info)
	case ActionStopAndRemove:
		if err := h.stopContainer(info); err != nil {
			return err
		}
		return h.removeContainer(info, false)
	case ActionRemove:
		return h.removeContainer(info, true)
	case ActionCancel:
		return nil
	default:
		return fmt.Errorf("unknown action: %v", action)
	}
}

// stopContainer stops a running container.
func (h *ActionHandler) stopContainer(info *ContainerInfo) error {
	fmt.Fprintf(h.writer, "%s⏸  Stopping container %s...%s\n", terminal.ColorCyan, info.Name, terminal.ColorReset)

	cmd := exec.Command("docker", "stop", info.ID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop container: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(h.writer, "%s✓ Container %s stopped successfully%s\n", terminal.ColorGreen, info.Name, terminal.ColorReset)
	return nil
}

// removeContainer removes a container (optionally with force).
func (h *ActionHandler) removeContainer(info *ContainerInfo, force bool) error {
	action := "Removing"
	if force {
		action = "Force removing"
	}

	fmt.Fprintf(h.writer, "%s🗑  %s container %s...%s\n", terminal.ColorCyan, action, info.Name, terminal.ColorReset)

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, info.ID)

	cmd := exec.Command("docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove container: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(h.writer, "%s✓ Container %s removed successfully%s\n", terminal.ColorGreen, info.Name, terminal.ColorReset)
	return nil
}
