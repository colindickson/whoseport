// Package docker provides Docker container detection and information retrieval.
package docker

// ContainerInfo represents comprehensive information about a Docker container.
type ContainerInfo struct {
	// Container identification
	ID         string `json:"id"`          // Full container ID
	ShortID    string `json:"short_id"`    // Short container ID (first 12 chars)
	Name       string `json:"name"`        // Container name
	Image      string `json:"image"`       // Image name
	ImageID    string `json:"image_id"`    // Image ID
	Command    string `json:"command"`     // Command running in container
	CreatedAt  string `json:"created_at"`  // Creation timestamp
	RunningFor string `json:"running_for"` // Duration running
	Status     string `json:"status"`      // Container status
	State      string `json:"state"`       // Container state (running, exited, etc.)

	// Port mappings
	Ports      []PortMapping `json:"ports"`       // Port mappings
	PortString string        `json:"port_string"` // Formatted port string

	// Network information
	Networks   []string `json:"networks"`    // Network names
	IPAddress  string   `json:"ip_address"`  // Primary IP address
	Gateway    string   `json:"gateway"`     // Gateway address
	MacAddress string   `json:"mac_address"` // MAC address

	// Resource usage
	CPUPercent string `json:"cpu_percent"` // CPU usage percentage
	MemUsage   string `json:"mem_usage"`   // Memory usage (e.g., "100MiB / 2GiB")
	MemPercent string `json:"mem_percent"` // Memory usage percentage
	NetIO      string `json:"net_io"`      // Network I/O
	BlockIO    string `json:"block_io"`    // Block I/O
	PIDs       string `json:"pids"`        // Number of PIDs

	// Container configuration
	Labels        map[string]string `json:"labels"`         // Container labels
	Mounts        []Mount           `json:"mounts"`         // Volume mounts
	RestartPolicy string            `json:"restart_policy"` // Restart policy
	Platform      string            `json:"platform"`       // Platform (linux/amd64, etc.)

	// Process information (from original whoseport detection)
	ProcessID  int    `json:"process_id"`  // The PID that led to container detection
	ProcessCmd string `json:"process_cmd"` // The process command
}

// PortMapping represents a Docker port mapping.
type PortMapping struct {
	HostIP        string `json:"host_ip"`        // Host IP (0.0.0.0, ::, etc.)
	HostPort      string `json:"host_port"`      // Port on host
	ContainerPort string `json:"container_port"` // Port in container
	Protocol      string `json:"protocol"`       // tcp or udp
}

// Mount represents a Docker volume mount.
type Mount struct {
	Type        string `json:"type"`        // bind, volume, tmpfs
	Source      string `json:"source"`      // Source path/volume
	Destination string `json:"destination"` // Destination in container
	Mode        string `json:"mode"`        // Mount mode (rw, ro)
}
