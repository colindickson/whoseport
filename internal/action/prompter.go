package action

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/bluehoodie/whoseport/internal/model"
)

// Prompter handles user prompts for process actions
// Following Dependency Inversion Principle: depends on io interfaces
type Prompter struct {
	reader      io.Reader
	writer      io.Writer
	colorBold   string
	colorYellow string
	colorCyan   string
	colorReset  string
}

// NewPrompter creates a new Prompter with stdin/stdout
func NewPrompter() *Prompter {
	return &Prompter{
		reader:      os.Stdin,
		writer:      os.Stdout,
		colorBold:   "\033[1m",
		colorYellow: "\033[33m",
		colorCyan:   "\033[36m",
		colorReset:  "\033[0m",
	}
}

// NewPrompterWithIO creates a new Prompter with custom reader/writer (for testing)
// Following Open/Closed Principle: extensible via constructor injection
func NewPrompterWithIO(reader io.Reader, writer io.Writer) *Prompter {
	return &Prompter{
		reader:      reader,
		writer:      writer,
		colorBold:   "\033[1m",
		colorYellow: "\033[33m",
		colorCyan:   "\033[36m",
		colorReset:  "\033[0m",
	}
}

// PromptKillAction prompts the user to select an action for the process
// Returns the selected signal and true if user wants to kill, or nil and false if cancelled
func (p *Prompter) PromptKillAction(info *model.ProcessInfo) (syscall.Signal, bool) {
	fmt.Fprintf(p.writer, "%s%s⚠️  Process %d (%s) - Select action:%s\n",
		p.colorBold, p.colorYellow, info.ID, info.Command, p.colorReset)
	fmt.Fprintf(p.writer, "  [1] SIGTERM (15) - Graceful termination\n")
	fmt.Fprintf(p.writer, "  [2] SIGKILL (9)  - Force kill (cannot be caught)\n")
	fmt.Fprintf(p.writer, "  [3] Cancel\n")
	fmt.Fprintf(p.writer, "%sChoice [3]:%s ", p.colorBold, p.colorReset)

	scanner := bufio.NewScanner(p.reader)

	for {
		if !scanner.Scan() {
			return 0, false
		}

		choice := strings.TrimSpace(scanner.Text())

		// Default to Cancel if empty
		if choice == "" {
			choice = "3"
		}

		switch choice {
		case "1":
			return syscall.SIGTERM, true
		case "2":
			return syscall.SIGKILL, true
		case "3":
			return 0, false
		default:
			fmt.Fprintf(p.writer, "%sInvalid choice. Please enter 1-3.%s\n", p.colorYellow, p.colorReset)
			fmt.Fprintf(p.writer, "%sChoice [3]:%s ", p.colorBold, p.colorReset)
		}
	}
}
