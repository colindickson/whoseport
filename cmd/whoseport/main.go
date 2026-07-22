package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/bluehoodie/whoseport/internal/action"
	"github.com/bluehoodie/whoseport/internal/display/docker"
	"github.com/bluehoodie/whoseport/internal/display/interactive"
	displayjson "github.com/bluehoodie/whoseport/internal/display/json"
	dockerpkg "github.com/bluehoodie/whoseport/internal/docker"
	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/process"
	"github.com/bluehoodie/whoseport/internal/procfs"
	"github.com/bluehoodie/whoseport/internal/terminal"
)

var (
	killFlag      bool
	termFlag      bool
	noInteractive bool
	jsonFlag      bool
)

// isNoServiceError checks if the error indicates no service was found on the port
func isNoServiceError(err error) bool {
	return errors.Is(err, process.ErrNoService)
}

func main() {
	flag.BoolVar(&killFlag, "kill", false, "Kill the process using the port (SIGKILL)")
	flag.BoolVar(&killFlag, "k", false, "Kill the process using the port (shorthand)")
	flag.BoolVar(&termFlag, "term", false, "Terminate the process using the port (SIGTERM)")
	flag.BoolVar(&termFlag, "t", false, "Terminate the process using the port (shorthand)")
	flag.BoolVar(&noInteractive, "no-interactive", false, "Disable interactive mode (show info only)")
	flag.BoolVar(&noInteractive, "n", false, "Disable interactive mode (shorthand)")
	flag.BoolVar(&jsonFlag, "json", false, "Output in JSON format")

	// Usage and argument errors go to stderr so that stdout carries only the
	// requested output; `whoseport --json bad-port | jq` must not be fed help text.
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "%s%sUsage of whoseport:%s\n", terminal.ColorBold, terminal.ColorCyan, terminal.ColorReset)
		fmt.Fprintf(w, "  %swhoseport [options] {port}%s\n\n", terminal.ColorWhite, terminal.ColorReset)
		fmt.Fprintf(w, "%sOptions:%s\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Fprintf(w, "  %s-k, --kill%s            Kill the process using the port (SIGKILL - force kill)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Fprintf(w, "  %s-t, --term%s            Terminate the process using the port (SIGTERM - graceful)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Fprintf(w, "  %s-n, --no-interactive%s  Disable interactive mode (show info only)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Fprintf(w, "  %s--json%s                Output in JSON format (implies -n)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Fprintf(w, "\n%sNote:%s Interactive mode is enabled by default\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Fprintf(w, "%sNote:%s On a Docker-backed port, -k and -t both stop the container (docker stop); neither removes it. Removing a container is only offered as an explicit choice in the interactive menu.\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Fprintf(w, "\n%sExample:%s\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Fprintf(w, "  whoseport 8080           # Show detailed info with interactive prompt\n")
		fmt.Fprintf(w, "  whoseport -n 8080        # Show info only, no prompt\n")
		fmt.Fprintf(w, "  whoseport -k 8080        # Force kill without prompting (SIGKILL)\n")
		fmt.Fprintf(w, "  whoseport -t 8080        # Gracefully terminate without prompting (SIGTERM)\n")
	}
	flag.Parse()

	// --json output must stay parseable, so it implies non-interactive mode:
	// otherwise the interactive kill prompt writes to the same stdout stream.
	if jsonFlag {
		noInteractive = true
	}

	// Validate flags
	if killFlag && termFlag {
		fmt.Fprintf(os.Stderr, "%serror:%s cannot use both -k/--kill and -t/--term flags together\n", terminal.ColorRed, terminal.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "%serror:%s missing port number\n", terminal.ColorRed, terminal.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	port, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%serror:%s port must be an integer\n", terminal.ColorRed, terminal.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	// Retrieve process information
	retriever := process.NewDefaultRetriever()
	processInfo, err := retriever.GetProcessByPort(port)
	if err != nil {
		// Check if it's a "no service found" case and provide a friendlier message
		if isNoServiceError(err) {
			fmt.Fprintf(os.Stderr, "No process is listening on port %d\n", port)
		} else {
			fmt.Fprintf(os.Stderr, "%serror:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
		}
		os.Exit(1)
	}

	// Enhance with detailed process information. Non-fatal: still show what we have.
	enhancer := procfs.NewProcessEnhancer()
	_ = enhancer.Enhance(processInfo)

	// Check if this is a Docker container process. Non-fatal: fall back to regular process display.
	detector := dockerpkg.NewDetector()
	isDocker, containerID, _ := detector.IsDockerRelated(processInfo, port)

	if isDocker && containerID != "" {
		// Docker container detected - use Docker-specific workflow
		handleDockerContainer(containerID, processInfo, port)
	} else {
		// Regular process - use existing workflow
		handleRegularProcess(processInfo, port)
	}
}

func handleDockerContainer(containerID string, processInfo *model.ProcessInfo, port int) {
	// Retrieve container information
	retriever := dockerpkg.NewRetriever()
	containerInfo, err := retriever.GetContainerInfo(containerID, processInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%serror:%s Failed to retrieve container info: %v\n", terminal.ColorRed, terminal.ColorReset, err)
		// Fall back to regular process handling
		handleRegularProcess(processInfo, port)
		return
	}

	// Display the container info
	if jsonFlag {
		out, err := json.MarshalIndent(containerInfo, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%serror:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	} else {
		displayer := docker.NewDisplayer()
		displayer.Display(containerInfo, port)
	}

	// Handle Docker actions
	actionHandler := dockerpkg.NewActionHandler()

	if killFlag || termFlag {
		// Direct action without prompting (equivalent to -k/-t flags).
		// -k/-t only stop the container (reversible) -- removing a container is
		// irreversible and is never a silent default, only an explicit choice
		// in the interactive menu (see PromptAction/ActionStopAndRemove below).
		if err := actionHandler.ExecuteAction(dockerpkg.ActionStop, containerInfo); err != nil {
			fmt.Fprintf(os.Stderr, "%s✗ Failed to execute action:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
	} else if !noInteractive {
		// Interactive mode - prompt for Docker action
		action := actionHandler.PromptAction(containerInfo)
		if action != dockerpkg.ActionCancel {
			if err := actionHandler.ExecuteAction(action, containerInfo); err != nil {
				fmt.Fprintf(os.Stderr, "%s✗ Failed to execute action:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
				os.Exit(1)
			}
		}
	}
}

func handleRegularProcess(processInfo *model.ProcessInfo, port int) {
	// Display the process info
	if jsonFlag {
		displayer := displayjson.NewDisplayer()
		if err := displayer.Display(processInfo); err != nil {
			fmt.Fprintf(os.Stderr, "%serror:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
	} else {
		displayer := interactive.NewDisplayer()
		displayer.Display(processInfo, port)
	}

	// Handle kill logic
	killer := action.NewKiller()
	if killFlag {
		// Force kill without prompting (SIGKILL)
		if err := killer.Kill(processInfo.ID, syscall.SIGKILL); err != nil {
			fmt.Fprintf(os.Stderr, "%s✗ Failed to kill process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%s✓ Successfully killed process %d with SIGKILL%s\n", terminal.ColorGreen, processInfo.ID, terminal.ColorReset)
	} else if termFlag {
		// Gracefully terminate without prompting (SIGTERM)
		if err := killer.Kill(processInfo.ID, syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "%s✗ Failed to terminate process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%s✓ Successfully terminated process %d with SIGTERM%s\n", terminal.ColorGreen, processInfo.ID, terminal.ColorReset)
	} else if !noInteractive {
		// Interactive mode - prompt for action in a single step
		prompter := action.NewPrompter()
		signal, shouldKill := prompter.PromptKillAction(processInfo)
		if shouldKill {
			// Send the selected signal
			if err := killer.Kill(processInfo.ID, signal); err != nil {
				fmt.Fprintf(os.Stderr, "%s✗ Failed to kill process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "%s✓ Successfully sent signal %v to process %d%s\n", terminal.ColorGreen, signal, processInfo.ID, terminal.ColorReset)
		}
	}
}
