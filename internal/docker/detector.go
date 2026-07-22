package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bluehoodie/whoseport/internal/model"
)

// dockerPortHandlerNames are the known executable names of processes that own a host
// port on behalf of a Docker container (as opposed to any process whose command line
// merely mentions "docker", e.g. one launched from a path containing that word).
var dockerPortHandlerNames = map[string]bool{
	"docker-proxy":       true, // Linux: forwards a published port to the container
	"com.docker.backend": true, // macOS: Docker Desktop's port-handling process
	"docker":             true,
}

// isDockerPortHandler reports whether command is one of the known Docker binaries that
// bind host ports for containers.
func isDockerPortHandler(command string) bool {
	name := strings.ToLower(filepath.Base(command))
	return dockerPortHandlerNames[name]
}

// Detector identifies if a process is related to a Docker container.
type Detector interface {
	// IsDockerRelated checks if the given process is Docker-related
	// Returns true and container ID if it's a Docker container process
	// port parameter is used to find containers by port mapping (useful for Docker Desktop on macOS)
	IsDockerRelated(info *model.ProcessInfo, port int) (bool, string, error)
}

// DefaultDetector implements Docker detection logic.
type DefaultDetector struct {
	dockerAvailable bool
}

// NewDetector creates a new Docker detector.
func NewDetector() Detector {
	detector := &DefaultDetector{}
	detector.checkDockerAvailability()
	return detector
}

// checkDockerAvailability verifies if Docker CLI is available.
func (d *DefaultDetector) checkDockerAvailability() {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	err := cmd.Run()
	d.dockerAvailable = err == nil
}

// IsDockerRelated determines if a process is Docker-related and returns the container ID.
func (d *DefaultDetector) IsDockerRelated(info *model.ProcessInfo, port int) (bool, string, error) {
	if !d.dockerAvailable {
		return false, "", nil
	}

	// Strategy 1: If the process is a known Docker port-handling binary, query Docker for
	// containers using this port. This works on both Linux and macOS (including Docker
	// Desktop where com.docker.backend handles ports).
	if isDockerPortHandler(info.Command) {
		containerID := d.findContainerByPort(port)
		if containerID != "" {
			return true, containerID, nil
		}
	}

	// Strategy 2: Check if the process is docker-proxy (Linux)
	// docker-proxy is the process that forwards ports from host to container
	if strings.Contains(info.Command, "docker-proxy") {
		containerID := d.findContainerFromDockerProxy(info)
		if containerID != "" {
			return true, containerID, nil
		}
	}

	// Strategy 3: Check if the process is running inside a container via cgroup (Linux)
	containerID, err := d.checkCgroupForContainer(info.ID)
	if err == nil && containerID != "" {
		return true, containerID, nil
	}

	return false, "", nil
}

// findContainerByPort finds a container by checking which container has a port mapping to the given host port.
// This works on both Linux and macOS Docker Desktop.
func (d *DefaultDetector) findContainerByPort(port int) string {
	// Use docker ps with format to get container IDs and ports
	cmd := exec.Command("docker", "ps", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse each line as JSON
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var container struct {
			ID    string `json:"ID"`
			Ports string `json:"Ports"`
		}

		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		// Check if this container has the port we're looking for
		// Ports format: "0.0.0.0:8080->80/tcp, :::8080->80/tcp"
		if d.containerHasPort(container.Ports, port) {
			return container.ID
		}
	}

	return ""
}

// containerHasPort checks if the ports string contains a TCP mapping from the given host port.
// whoseport only ever looks for TCP listeners (lsof is filtered to LISTEN state), so a container
// publishing the same port number over UDP must not be treated as a match.
func (d *DefaultDetector) containerHasPort(portsStr string, port int) bool {
	portStr := strconv.Itoa(port)

	// Split by comma to handle multiple port mappings
	portMappings := strings.Split(portsStr, ",")

	for _, mapping := range portMappings {
		mapping = strings.TrimSpace(mapping)

		// Look for patterns like:
		// "0.0.0.0:8080->80/tcp"
		// ":::8080->80/tcp"
		// "*:8080->80/tcp"
		// "8080->80/tcp"

		// Split into host part and "containerPort/proto" (the part after ->)
		hostPart, containerPart, ok := strings.Cut(mapping, "->")
		if !ok {
			continue
		}

		// Only TCP mappings are relevant.
		_, proto, ok := strings.Cut(containerPart, "/")
		if !ok || !strings.EqualFold(proto, "tcp") {
			continue
		}

		// Host part can be:
		// "0.0.0.0:8080"
		// ":::8080"
		// "*:8080"
		// "8080"

		// Extract port from host part
		hostPort := hostPart
		if lastColon := strings.LastIndex(hostPart, ":"); lastColon != -1 {
			hostPort = hostPart[lastColon+1:]
		}

		if hostPort == portStr {
			return true
		}
	}

	return false
}

// findContainerFromDockerProxy attempts to find the container ID from docker-proxy command.
// docker-proxy command typically looks like:
// /usr/bin/docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 8080 -container-ip 172.17.0.2 -container-port 8080
func (d *DefaultDetector) findContainerFromDockerProxy(info *model.ProcessInfo) string {
	// Extract container IP from docker-proxy arguments
	containerIP := d.extractContainerIPFromProxy(info)
	if containerIP == "" {
		return ""
	}

	// Find container by IP address
	return d.findContainerByIP(containerIP)
}

// extractContainerIPFromProxy parses docker-proxy command line for container IP.
func (d *DefaultDetector) extractContainerIPFromProxy(info *model.ProcessInfo) string {
	// Parse from FullCommand which contains all arguments
	cmdline := info.FullCommand

	// Look for -container-ip argument
	re := regexp.MustCompile(`-container-ip\s+([0-9.]+)`)
	matches := re.FindStringSubmatch(cmdline)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Also try parsing from /proc/{pid}/cmdline if FullCommand didn't work
	cmdlineFile := fmt.Sprintf("/proc/%d/cmdline", info.ID)
	data, err := os.ReadFile(cmdlineFile)
	if err != nil {
		return ""
	}

	// cmdline is null-separated
	parts := strings.Split(string(data), "\x00")
	for i, part := range parts {
		if part == "-container-ip" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// findContainerByIP finds a container by its IP address.
func (d *DefaultDetector) findContainerByIP(ip string) string {
	// Use docker inspect to find container with this IP
	cmd := exec.Command("docker", "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, containerID := range containerIDs {
		if containerID == "" {
			continue
		}

		// Check if this container has the matching IP
		cmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerID)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		containerIPAddr := strings.TrimSpace(string(output))
		if containerIPAddr == ip {
			return containerID
		}
	}

	return ""
}

// checkCgroupForContainer checks if the process is running in a container by examining cgroup.
func (d *DefaultDetector) checkCgroupForContainer(pid int) (string, error) {
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	file, err := os.Open(cgroupFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Pattern for extracting container ID from cgroup paths
	// Docker cgroup paths typically look like:
	// 12:memory:/docker/8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f
	// or
	// 12:memory:/system.slice/docker-8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f.scope
	dockerPattern := regexp.MustCompile(`/docker[/-]([a-f0-9]{64})`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := dockerPattern.FindStringSubmatch(line)
		if len(matches) >= 2 {
			return matches[1], nil
		}
	}

	return "", scanner.Err()
}
