package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/bluehoodie/whoseport/internal/model"
)

// Retriever retrieves detailed Docker container information.
type Retriever interface {
	// GetContainerInfo retrieves detailed information about a container
	GetContainerInfo(containerID string, processInfo *model.ProcessInfo) (*ContainerInfo, error)
}

// DefaultRetriever implements container information retrieval.
type DefaultRetriever struct{}

// NewRetriever creates a new Docker container retriever.
func NewRetriever() Retriever {
	return &DefaultRetriever{}
}

// GetContainerInfo retrieves comprehensive container information.
func (r *DefaultRetriever) GetContainerInfo(containerID string, processInfo *model.ProcessInfo) (*ContainerInfo, error) {
	info := &ContainerInfo{
		ID:         containerID,
		ShortID:    containerID,
		ProcessID:  processInfo.ID,
		ProcessCmd: processInfo.Command,
	}

	// Truncate to short ID if it's a full ID
	if len(containerID) > 12 {
		info.ShortID = containerID[:12]
	}

	// Get detailed inspect data
	if err := r.populateInspectData(info); err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get real-time stats (non-blocking)
	// We don't fail if stats aren't available
	_ = r.populateStatsData(info)

	return info, nil
}

// inspectData mirrors the subset of `docker inspect` JSON output that whoseport uses.
type inspectData struct {
	Name    string `json:"Name"`
	Created string `json:"Created"`
	Image   string `json:"Image"`
	Config  struct {
		Image  string            `json:"Image"`
		Cmd    []string          `json:"Cmd"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	HostConfig struct {
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports    map[string][]inspectPortBinding `json:"Ports"`
		Networks map[string]inspectNetwork       `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts   []inspectMount `json:"Mounts"`
	Platform string         `json:"Platform"`
}

type inspectPortBinding struct {
	HostIp   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type inspectNetwork struct {
	IPAddress  string `json:"IPAddress"`
	Gateway    string `json:"Gateway"`
	MacAddress string `json:"MacAddress"`
}

type inspectMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

// populateInspectData uses docker inspect to get container details.
func (r *DefaultRetriever) populateInspectData(info *ContainerInfo) error {
	cmd := exec.Command("docker", "inspect", info.ID)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	return r.parseInspectOutput(info, output)
}

// parseInspectOutput decodes the JSON produced by `docker inspect` into info.
// Split out from populateInspectData so it can be tested without shelling out.
func (r *DefaultRetriever) parseInspectOutput(info *ContainerInfo, output []byte) error {
	var inspected []inspectData
	if err := json.Unmarshal(output, &inspected); err != nil {
		return err
	}

	if len(inspected) == 0 {
		return fmt.Errorf("no inspect data returned")
	}

	data := inspected[0]

	info.Name = strings.TrimPrefix(data.Name, "/")
	info.Image = data.Config.Image
	info.Command = strings.Join(data.Config.Cmd, " ")
	info.Labels = data.Config.Labels

	info.State = data.State.Status
	info.Status = data.State.Status
	info.CreatedAt = data.State.StartedAt
	if t, err := time.Parse(time.RFC3339Nano, data.State.StartedAt); err == nil {
		info.RunningFor = formatDuration(time.Since(t))
	}

	info.ImageID = data.Image
	if len(info.ImageID) > 19 && strings.HasPrefix(info.ImageID, "sha256:") {
		info.ImageID = info.ImageID[7:19] // Short hash
	}

	info.RestartPolicy = data.HostConfig.RestartPolicy.Name

	info.Ports = r.parsePortMappings(data.NetworkSettings.Ports)
	info.PortString = r.formatPortString(info.Ports)

	// Network iteration order from a Go map is random; sort names so the
	// reported "primary" network (and its IP/gateway/MAC) is deterministic.
	netNames := make([]string, 0, len(data.NetworkSettings.Networks))
	for name := range data.NetworkSettings.Networks {
		netNames = append(netNames, name)
	}
	sort.Strings(netNames)
	info.Networks = netNames
	if len(netNames) > 0 {
		primary := data.NetworkSettings.Networks[netNames[0]]
		info.IPAddress = primary.IPAddress
		info.Gateway = primary.Gateway
		info.MacAddress = primary.MacAddress
	}

	info.Mounts = make([]Mount, 0, len(data.Mounts))
	for _, m := range data.Mounts {
		mode := "ro"
		if m.RW {
			mode = "rw"
		}
		info.Mounts = append(info.Mounts, Mount{
			Type:        m.Type,
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        mode,
		})
	}

	info.Platform = data.Platform

	return nil
}

// populateStatsData uses docker stats to get real-time resource usage.
func (r *DefaultRetriever) populateStatsData(info *ContainerInfo) error {
	// Use --no-stream to get a single snapshot
	cmd := exec.Command("docker", "stats", "--no-stream", "--format",
		"{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.NetIO}}|{{.BlockIO}}|{{.PIDs}}",
		info.ID)

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	line := strings.TrimSpace(string(output))
	parts := strings.Split(line, "|")

	if len(parts) >= 6 {
		info.CPUPercent = strings.TrimSpace(parts[0])
		info.MemUsage = strings.TrimSpace(parts[1])
		info.MemPercent = strings.TrimSpace(parts[2])
		info.NetIO = strings.TrimSpace(parts[3])
		info.BlockIO = strings.TrimSpace(parts[4])
		info.PIDs = strings.TrimSpace(parts[5])
	}

	return nil
}

// parsePortMappings converts Docker port map to PortMapping structs.
func (r *DefaultRetriever) parsePortMappings(ports map[string][]inspectPortBinding) []PortMapping {
	mappings := make([]PortMapping, 0)

	// Map iteration order is random; sort container ports for deterministic output.
	containerPorts := make([]string, 0, len(ports))
	for containerPort := range ports {
		containerPorts = append(containerPorts, containerPort)
	}
	sort.Strings(containerPorts)

	for _, containerPort := range containerPorts {
		for _, binding := range ports[containerPort] {
			mapping := PortMapping{
				HostIP:   binding.HostIp,
				HostPort: binding.HostPort,
			}

			// Parse container port (format: "8080/tcp")
			portParts := strings.Split(containerPort, "/")
			if len(portParts) >= 1 {
				mapping.ContainerPort = portParts[0]
			}
			if len(portParts) >= 2 {
				mapping.Protocol = portParts[1]
			}

			mappings = append(mappings, mapping)
		}
	}

	return mappings
}

// formatPortString creates a human-readable port mapping string.
func (r *DefaultRetriever) formatPortString(ports []PortMapping) string {
	if len(ports) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		hostPart := p.HostPort
		if p.HostIP != "" && p.HostIP != "0.0.0.0" {
			hostPart = p.HostIP + ":" + p.HostPort
		}

		part := fmt.Sprintf("%s->%s/%s", hostPart, p.ContainerPort, p.Protocol)
		parts = append(parts, part)
	}

	return strings.Join(parts, ", ")
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%d hours %d minutes", hours, minutes)
		}
		return fmt.Sprintf("%d hours", hours)
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%d days %d hours", days, hours)
	}
	return fmt.Sprintf("%d days", days)
}
