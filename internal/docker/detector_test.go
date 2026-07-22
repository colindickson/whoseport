package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluehoodie/whoseport/internal/model"
)

func TestExtractContainerIPFromProxy(t *testing.T) {
	detector := &DefaultDetector{}

	tests := []struct {
		name        string
		fullCommand string
		expected    string
	}{
		{
			name:        "valid docker-proxy command",
			fullCommand: "/usr/bin/docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 8080 -container-ip 172.17.0.2 -container-port 8080",
			expected:    "172.17.0.2",
		},
		{
			name:        "docker-proxy with ipv6",
			fullCommand: "/usr/bin/docker-proxy -proto tcp -host-ip :: -host-port 8080 -container-ip 172.17.0.3 -container-port 80",
			expected:    "172.17.0.3",
		},
		{
			name:        "no container-ip in command",
			fullCommand: "/usr/bin/docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 8080",
			expected:    "",
		},
		{
			name:        "empty command",
			fullCommand: "",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &model.ProcessInfo{
				Command:     "docker-proxy",
				FullCommand: tt.fullCommand,
			}

			result := detector.extractContainerIPFromProxy(info)
			if result != tt.expected {
				t.Errorf("extractContainerIPFromProxy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsDockerPortHandler(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{name: "docker-proxy", command: "docker-proxy", expected: true},
		{name: "docker-proxy with path", command: "/usr/bin/docker-proxy", expected: true},
		{name: "com.docker.backend", command: "com.docker.backend", expected: true},
		{name: "bare docker", command: "docker", expected: true},
		{name: "case insensitive", command: "Docker-Proxy", expected: true},
		{name: "unrelated binary launched from a path containing docker", command: "node", expected: false},
		{name: "unrelated binary name merely containing docker", command: "dockerized-app", expected: false},
		{name: "empty command", command: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDockerPortHandler(tt.command)
			if result != tt.expected {
				t.Errorf("isDockerPortHandler(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestDockerAvailableProbesOnlyOnce(t *testing.T) {
	// Fake "docker" binary on PATH that records each invocation and succeeds.
	dir := t.TempDir()
	countFile := filepath.Join(dir, "count")
	script := "#!/bin/sh\necho x >> " + countFile + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake docker script: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &DefaultDetector{}
	if !d.dockerAvailable() {
		t.Fatalf("expected dockerAvailable() to be true with fake docker on PATH")
	}
	if !d.dockerAvailable() {
		t.Fatalf("expected cached dockerAvailable() to still be true")
	}

	data, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatalf("fake docker script was never invoked: %v", err)
	}
	invocations := strings.Count(string(data), "x\n")
	if invocations != 1 {
		t.Errorf("expected docker to be probed exactly once, got %d invocations", invocations)
	}
}

func TestCheckCgroupForContainer(t *testing.T) {
	detector := &DefaultDetector{}

	// Note: This test will only work on systems with /proc filesystem
	// For a process running in Docker, we expect to find a container ID
	// For a regular process, we expect empty string

	// Test with a non-existent PID (should return error or empty string)
	containerID, err := detector.checkCgroupForContainer(999999)
	if err == nil && containerID != "" {
		t.Errorf("Expected no container ID for non-existent PID, got: %s", containerID)
	}
}

func TestContainerHasPort(t *testing.T) {
	detector := &DefaultDetector{}

	tests := []struct {
		name     string
		portsStr string
		port     int
		expected bool
	}{
		{
			name:     "single port mapping with 0.0.0.0",
			portsStr: "0.0.0.0:8080->80/tcp",
			port:     8080,
			expected: true,
		},
		{
			name:     "single port mapping with IPv6",
			portsStr: ":::8080->80/tcp",
			port:     8080,
			expected: true,
		},
		{
			name:     "multiple port mappings",
			portsStr: "0.0.0.0:8080->80/tcp, :::8080->80/tcp, 0.0.0.0:9090->90/tcp",
			port:     9090,
			expected: true,
		},
		{
			name:     "port not in list",
			portsStr: "0.0.0.0:8080->80/tcp, 0.0.0.0:9090->90/tcp",
			port:     7070,
			expected: false,
		},
		{
			name:     "empty ports string",
			portsStr: "",
			port:     8080,
			expected: false,
		},
		{
			name:     "port mapping with wildcard",
			portsStr: "*:8080->80/tcp",
			port:     8080,
			expected: true,
		},
		{
			name:     "check first port in multiple mappings",
			portsStr: "0.0.0.0:3000->3000/tcp, 0.0.0.0:8080->80/tcp",
			port:     3000,
			expected: true,
		},
		{
			name:     "udp mapping does not match a tcp lookup",
			portsStr: "0.0.0.0:8080->80/udp",
			port:     8080,
			expected: false,
		},
		{
			name:     "udp mapping among mixed protocols does not match",
			portsStr: "0.0.0.0:8080->80/udp, :::8080->80/udp",
			port:     8080,
			expected: false,
		},
		{
			name:     "tcp mapping matches even when a udp mapping for the same port exists",
			portsStr: "0.0.0.0:8080->80/udp, 0.0.0.0:8080->80/tcp",
			port:     8080,
			expected: true,
		},
		{
			name:     "bare port with no host ip still matches on tcp",
			portsStr: "8080->80/tcp",
			port:     8080,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.containerHasPort(tt.portsStr, tt.port)
			if result != tt.expected {
				t.Errorf("containerHasPort(%q, %d) = %v, want %v", tt.portsStr, tt.port, result, tt.expected)
			}
		})
	}
}
