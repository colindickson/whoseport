package docker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	reader      io.Reader
	writer      io.Writer
	colorBold   string
	colorYellow string
	colorCyan   string
	colorRed    string
	colorGreen  string
	colorReset  string
}

// NewActionHandler creates a new Docker action handler.
func NewActionHandler() *ActionHandler {
	return &ActionHandler{
		reader:      os.Stdin,
		writer:      os.Stdout,
		colorBold:   "\033[1m",
		colorYellow: "\033[33m",
		colorCyan:   "\033[36m",
		colorRed:    "\033[31m",
		colorGreen:  "\033[32m",
		colorReset:  "\033[0m",
	}
}

// NewActionHandlerWithIO creates a new action handler with custom reader/writer (for testing).
func NewActionHandlerWithIO(reader io.Reader, writer io.Writer) *ActionHandler {
	return &ActionHandler{
		reader:      reader,
		writer:      writer,
		colorBold:   "\033[1m",
		colorYellow: "\033[33m",
		colorCyan:   "\033[36m",
		colorRed:    "\033[31m",
		colorGreen:  "\033[32m",
		colorReset:  "\033[0m",
	}
}

// PromptAction prompts the user to select an action for the Docker container.
func (h *ActionHandler) PromptAction(info *ContainerInfo) Action {
	fmt.Fprintf(h.writer, "%s%s🐳 Container %s (%s) - Select action:%s\n",
		h.colorBold, h.colorYellow, info.Name, info.ShortID, h.colorReset)
	fmt.Fprintf(h.writer, "  [1] Stop container (docker stop)\n")
	fmt.Fprintf(h.writer, "  [2] Stop and remove container (docker stop + docker rm)\n")
	fmt.Fprintf(h.writer, "  [3] Force remove running container (docker rm -f)\n")
	fmt.Fprintf(h.writer, "  [4] Cancel\n")
	fmt.Fprintf(h.writer, "%sChoice [4]:%s ", h.colorBold, h.colorReset)

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
			fmt.Fprintf(h.writer, "%sInvalid choice. Please enter 1-4.%s\n", h.colorYellow, h.colorReset)
			fmt.Fprintf(h.writer, "%sChoice [4]:%s ", h.colorBold, h.colorReset)
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
	fmt.Fprintf(h.writer, "%s⏸  Stopping container %s...%s\n", h.colorCyan, info.Name, h.colorReset)

	cmd := exec.Command("docker", "stop", info.ID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop container: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(h.writer, "%s✓ Container %s stopped successfully%s\n", h.colorGreen, info.Name, h.colorReset)
	return nil
}

// removeContainer removes a container (optionally with force).
func (h *ActionHandler) removeContainer(info *ContainerInfo, force bool) error {
	action := "Removing"
	if force {
		action = "Force removing"
	}

	fmt.Fprintf(h.writer, "%s🗑  %s container %s...%s\n", h.colorCyan, action, info.Name, h.colorReset)

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, info.ID)

	cmd := exec.Command("docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove container: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(h.writer, "%s✓ Container %s removed successfully%s\n", h.colorGreen, info.Name, h.colorReset)
	return nil
}
