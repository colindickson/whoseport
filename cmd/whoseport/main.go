package main

import (
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

	flag.Usage = func() {
		fmt.Printf("%s%sUsage of whoseport:%s\n", terminal.ColorBold, terminal.ColorCyan, terminal.ColorReset)
		fmt.Printf("  %swhoseport [options] {port}%s\n\n", terminal.ColorWhite, terminal.ColorReset)
		fmt.Printf("%sOptions:%s\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Printf("  %s-k, --kill%s            Kill the process using the port (SIGKILL - force kill)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Printf("  %s-t, --term%s            Terminate the process using the port (SIGTERM - graceful)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Printf("  %s-n, --no-interactive%s  Disable interactive mode (show info only)\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Printf("  %s--json%s                Output in JSON format\n", terminal.ColorYellow, terminal.ColorReset)
		fmt.Printf("\n%sNote:%s Interactive mode is enabled by default\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Printf("\n%sExample:%s\n", terminal.ColorBold, terminal.ColorReset)
		fmt.Printf("  whoseport 8080           # Show detailed info with interactive prompt\n")
		fmt.Printf("  whoseport -n 8080        # Show info only, no prompt\n")
		fmt.Printf("  whoseport -k 8080        # Force kill without prompting (SIGKILL)\n")
		fmt.Printf("  whoseport -t 8080        # Gracefully terminate without prompting (SIGTERM)\n")
	}
	flag.Parse()

	// Validate flags
	if killFlag && termFlag {
		fmt.Printf("%serror:%s cannot use both -k/--kill and -t/--term flags together\n", terminal.ColorRed, terminal.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() < 1 {
		fmt.Printf("%serror:%s missing port number\n", terminal.ColorRed, terminal.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	port, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		fmt.Printf("%serror:%s port must be an integer\n", terminal.ColorRed, terminal.ColorReset)
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

	// Enhance with detailed process information
	enhancer := procfs.NewProcessEnhancer()
	enhancer.Enhance(processInfo)

	// Check if this is a Docker container process
	detector := dockerpkg.NewDetector()
	isDocker, containerID, err := detector.IsDockerRelated(processInfo, port)

	if err == nil && isDocker && containerID != "" {
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
		// For JSON mode, we could output the container info as JSON
		// For now, fall back to process info JSON
		displayer := displayjson.NewDisplayer()
		if err := displayer.Display(processInfo); err != nil {
			fmt.Fprintf(os.Stderr, "%serror:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
	} else {
		displayer := docker.NewDisplayer()
		displayer.Display(containerInfo, port)
	}

	// Handle Docker actions
	actionHandler := dockerpkg.NewActionHandler()

	if killFlag || termFlag {
		// Direct action without prompting (equivalent to -k/-t flags)
		// For Docker, -k/-t means stop and remove the container
		var action dockerpkg.Action
		if killFlag {
			action = dockerpkg.ActionStopAndRemove
		} else {
			action = dockerpkg.ActionStop
		}

		if err := actionHandler.ExecuteAction(action, containerInfo); err != nil {
			fmt.Printf("%s✗ Failed to execute action:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
	} else if !noInteractive {
		// Interactive mode - prompt for Docker action
		action := actionHandler.PromptAction(containerInfo)
		if action != dockerpkg.ActionCancel {
			if err := actionHandler.ExecuteAction(action, containerInfo); err != nil {
				fmt.Printf("%s✗ Failed to execute action:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
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
			fmt.Printf("%s✗ Failed to kill process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Successfully killed process %d with SIGKILL%s\n", terminal.ColorGreen, processInfo.ID, terminal.ColorReset)
	} else if termFlag {
		// Gracefully terminate without prompting (SIGTERM)
		if err := killer.Kill(processInfo.ID, syscall.SIGTERM); err != nil {
			fmt.Printf("%s✗ Failed to terminate process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Successfully terminated process %d with SIGTERM%s\n", terminal.ColorGreen, processInfo.ID, terminal.ColorReset)
	} else if !noInteractive {
		// Interactive mode - prompt for action in a single step
		prompter := action.NewPrompter()
		signal, shouldKill := prompter.PromptKillAction(processInfo)
		if shouldKill {
			// Send the selected signal
			if err := killer.Kill(processInfo.ID, signal); err != nil {
				fmt.Printf("%s✗ Failed to kill process:%s %v\n", terminal.ColorRed, terminal.ColorReset, err)
				os.Exit(1)
			}
			fmt.Printf("%s✓ Successfully sent signal %v to process %d%s\n", terminal.ColorGreen, signal, processInfo.ID, terminal.ColorReset)
		}
	}
}
