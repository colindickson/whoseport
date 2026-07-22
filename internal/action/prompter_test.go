package action

import (
	"bytes"
	"strings"
	"syscall"
	"testing"

	"github.com/bluehoodie/whoseport/internal/model"
)

// TestPromptKillActionSIGTERM tests selecting SIGTERM from the combined menu
func TestPromptKillActionSIGTERM(t *testing.T) {
	input := "1\n" // User selects option 1 (SIGTERM)
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	signal, shouldKill := prompter.PromptKillAction(info)

	if !shouldKill {
		t.Error("PromptKillAction() should return true for SIGTERM selection")
	}
	if signal != syscall.SIGTERM {
		t.Errorf("Expected SIGTERM, got %v", signal)
	}

	// Verify menu was displayed with process info
	outputStr := output.String()
	if !strings.Contains(outputStr, "12345") {
		t.Error("Menu should display process ID")
	}
	if !strings.Contains(outputStr, "test-process") {
		t.Error("Menu should display process command")
	}
	if !strings.Contains(outputStr, "SIGTERM") {
		t.Error("Menu should contain SIGTERM option")
	}
}

// TestPromptKillActionSIGKILL tests selecting SIGKILL from the combined menu
func TestPromptKillActionSIGKILL(t *testing.T) {
	input := "2\n" // User selects option 2 (SIGKILL)
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	signal, shouldKill := prompter.PromptKillAction(info)

	if !shouldKill {
		t.Error("PromptKillAction() should return true for SIGKILL selection")
	}
	if signal != syscall.SIGKILL {
		t.Errorf("Expected SIGKILL, got %v", signal)
	}
}

// TestPromptKillActionCancel tests selecting Cancel from the combined menu
func TestPromptKillActionCancel(t *testing.T) {
	input := "3\n" // User selects option 3 (Cancel)
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	_, shouldKill := prompter.PromptKillAction(info)

	if shouldKill {
		t.Error("PromptKillAction() should return false when user cancels")
	}
}

// TestPromptKillActionDefaultCancel tests that empty input defaults to Cancel
func TestPromptKillActionDefaultCancel(t *testing.T) {
	input := "\n" // User presses Enter without selecting anything
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	_, shouldKill := prompter.PromptKillAction(info)

	if shouldKill {
		t.Error("PromptKillAction() should default to Cancel when user presses Enter")
	}
}

// TestPromptKillActionInvalidOption tests handling of invalid menu selection
func TestPromptKillActionInvalidOption(t *testing.T) {
	input := "99\n1\n" // Invalid option first, then valid option (SIGTERM)
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	signal, shouldKill := prompter.PromptKillAction(info)

	if !shouldKill {
		t.Error("PromptKillAction() should return true after recovering from invalid input")
	}
	if signal != syscall.SIGTERM {
		t.Errorf("Expected SIGTERM after recovery, got %v", signal)
	}

	// Verify error message was shown
	outputStr := output.String()
	if !strings.Contains(outputStr, "Invalid") && !strings.Contains(outputStr, "invalid") {
		t.Error("Should display error message for invalid input")
	}
}

// TestPromptKillActionEOF tests handling of EOF (e.g., Ctrl+D)
func TestPromptKillActionEOF(t *testing.T) {
	input := "" // Empty input simulates EOF
	reader := strings.NewReader(input)
	output := &bytes.Buffer{}

	prompter := NewPrompterWithIO(reader, output)
	info := &model.ProcessInfo{
		ID:      12345,
		Command: "test-process",
	}

	_, shouldKill := prompter.PromptKillAction(info)

	if shouldKill {
		t.Error("PromptKillAction() should return false on EOF")
	}
}
