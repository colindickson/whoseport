package docker

import (
	"fmt"
	"strings"

	"github.com/bluehoodie/whoseport/internal/display/format"
	"github.com/bluehoodie/whoseport/internal/docker"
	"github.com/bluehoodie/whoseport/internal/terminal"
)

// Displayer outputs Docker ContainerInfo as interactive colorized UI.
type Displayer struct {
	theme terminal.Theme
	width int
}

// NewDisplayer creates a new Docker container displayer.
func NewDisplayer() *Displayer {
	return &Displayer{
		theme: terminal.DefaultTheme(),
		width: terminal.Width(),
	}
}

// Display outputs the ContainerInfo as interactive UI.
func (d *Displayer) Display(info *docker.ContainerInfo, port int) {
	// Banner - clearly indicate this is a Docker container
	d.printDockerBanner(fmt.Sprintf("PORT %d → DOCKER CONTAINER", port))

	// Section 1: Container Identity
	d.printModernSection("🐳 CONTAINER IDENTITY")
	d.printEnhancedField("Container Name", info.Name, terminal.ColorBrightCyan, "📦")
	d.printEnhancedField("Container ID", info.ShortID, terminal.ColorLavender, "")
	if len(info.ID) > 12 {
		d.printEnhancedField("  Full ID", info.ID, terminal.ColorDim, "")
	}

	// State with color coding
	stateColor := d.getStateColor(info.State)
	stateEmoji := d.getStateEmoji(info.State)
	d.printEnhancedField("State", fmt.Sprintf("%s %s", stateEmoji, strings.ToUpper(info.State)), stateColor, "")

	if info.RunningFor != "" {
		d.printEnhancedField("Running For", info.RunningFor, terminal.ColorLime, "⏱")
	}

	// Section 2: Image Information
	d.printModernSection("📀 IMAGE")
	d.printEnhancedField("Image", info.Image, terminal.ColorBrightGreen, "")
	if info.ImageID != "" {
		d.printEnhancedField("Image ID", info.ImageID, terminal.ColorDim, "")
	}
	if info.Platform != "" {
		d.printEnhancedField("Platform", info.Platform, terminal.ColorPeach, "")
	}
	if info.Command != "" {
		d.printEnhancedField("Command", format.Truncate(info.Command, 60), terminal.ColorOrange, "⚙️")
	}

	// Section 3: Network & Ports
	d.printModernSection("🌐 NETWORK & PORTS")

	// Show the specific port mapping that matched
	if len(info.Ports) > 0 {
		d.printEnhancedField("Port Mappings", "", terminal.ColorBrightCyan, "")
		for _, portMap := range info.Ports {
			var hostDisplay string
			if portMap.HostIP != "" && portMap.HostIP != "0.0.0.0" && portMap.HostIP != "::" {
				hostDisplay = portMap.HostIP + ":" + portMap.HostPort
			} else {
				hostDisplay = "*:" + portMap.HostPort
			}

			// Highlight the port we searched for
			portColor := terminal.ColorMint
			emoji := "  "
			if portMap.HostPort == fmt.Sprintf("%d", port) {
				portColor = terminal.ColorBrightGreen
				emoji = "👉"
			}

			d.printEnhancedField(
				fmt.Sprintf("  %s:%s", hostDisplay, portMap.ContainerPort),
				strings.ToUpper(portMap.Protocol),
				portColor,
				emoji,
			)
		}
	}

	if info.IPAddress != "" {
		d.printEnhancedField("IP Address", info.IPAddress, terminal.ColorSkyBlue, "🔗")
	}
	if info.Gateway != "" {
		d.printEnhancedField("Gateway", info.Gateway, terminal.ColorDim, "")
	}
	if info.MacAddress != "" {
		d.printEnhancedField("MAC Address", info.MacAddress, terminal.ColorDim, "")
	}

	if len(info.Networks) > 0 {
		d.printEnhancedField("Networks", strings.Join(info.Networks, ", "), terminal.ColorLavender, "")
	}

	// Section 4: Resource Usage
	if info.CPUPercent != "" || info.MemUsage != "" {
		d.printModernSection("📊 RESOURCE USAGE")

		if info.CPUPercent != "" {
			d.printEnhancedField("CPU Usage", info.CPUPercent, terminal.ColorOrange, "⚡")
		}

		if info.MemUsage != "" {
			memDisplay := info.MemUsage
			if info.MemPercent != "" {
				memDisplay += " (" + info.MemPercent + ")"
			}
			d.printEnhancedField("Memory Usage", memDisplay, terminal.ColorLime, "💾")
		}

		if info.NetIO != "" {
			d.printEnhancedField("Network I/O", info.NetIO, terminal.ColorBrightCyan, "📡")
		}

		if info.BlockIO != "" {
			d.printEnhancedField("Block I/O", info.BlockIO, terminal.ColorCoral, "💿")
		}

		if info.PIDs != "" {
			d.printEnhancedField("PIDs", info.PIDs, terminal.ColorViolet, "")
		}
	}

	// Section 5: Configuration
	d.printModernSection("⚙️  CONFIGURATION")

	if info.RestartPolicy != "" {
		d.printEnhancedField("Restart Policy", info.RestartPolicy, terminal.ColorGold, "")
	}

	// Mounts
	if len(info.Mounts) > 0 {
		d.printEnhancedField("Mounts", fmt.Sprintf("%d volume(s)", len(info.Mounts)), terminal.ColorCyan, "📂")
		for i, mount := range info.Mounts {
			if i < 5 { // Show first 5 mounts
				src := mount.Source
				if len(src) > 40 {
					src = "..." + src[len(src)-37:]
				}
				dst := mount.Destination
				if len(dst) > 30 {
					dst = "..." + dst[len(dst)-27:]
				}

				mountLine := fmt.Sprintf("%s → %s (%s, %s)", src, dst, mount.Type, mount.Mode)
				fmt.Printf("    %s┃%s %s%s%s\n",
					terminal.ColorBrightBlue, terminal.ColorReset,
					terminal.ColorDim, mountLine, terminal.ColorReset)
			}
		}
		if len(info.Mounts) > 5 {
			fmt.Printf("    %s┗━ and %d more...%s\n", terminal.ColorDim, len(info.Mounts)-5, terminal.ColorReset)
		}
	}

	// Labels
	if len(info.Labels) > 0 {
		d.printEnhancedField("Labels", fmt.Sprintf("%d label(s)", len(info.Labels)), terminal.ColorLavender, "🏷")
		count := 0
		for key, value := range info.Labels {
			if count < 5 { // Show first 5 labels
				labelText := fmt.Sprintf("%s=%s", key, value)
				if len(labelText) > 70 {
					labelText = labelText[:67] + "..."
				}
				fmt.Printf("    %s┃%s %s%s%s\n",
					terminal.ColorBrightBlue, terminal.ColorReset,
					terminal.ColorDim, labelText, terminal.ColorReset)
				count++
			}
		}
		if len(info.Labels) > 5 {
			fmt.Printf("    %s┗━ and %d more...%s\n", terminal.ColorDim, len(info.Labels)-5, terminal.ColorReset)
		}
	}

	// Section 6: Underlying Process (for context)
	if info.ProcessID > 0 {
		d.printModernSection("🔧 UNDERLYING PROCESS")
		d.printEnhancedField("Process ID", fmt.Sprintf("%d", info.ProcessID), terminal.ColorDim, "")
		if info.ProcessCmd != "" {
			d.printEnhancedField("Process Command", info.ProcessCmd, terminal.ColorDim, "")
		}
		fmt.Printf("  %s%sNote:%s The process above is the Docker proxy/container process.%s\n",
			terminal.ColorDim, terminal.ColorBold, terminal.ColorReset, terminal.ColorReset)
		fmt.Printf("  %sActions below will affect the %scontainer%s, not just the process.%s\n",
			terminal.ColorDim, terminal.ColorBold, terminal.ColorReset, terminal.ColorReset)
	}

	d.printGradientDivider()
}

func (d *Displayer) printDockerBanner(text string) {
	textLen := format.VisualWidth(text)
	emojiWidth := 0 // No emoji in text itself

	innerWidth := d.width - 2
	if innerWidth < textLen+emojiWidth {
		innerWidth = textLen + emojiWidth
	}
	leftPadding := (innerWidth - textLen - emojiWidth) / 2
	rightPadding := innerWidth - textLen - emojiWidth - leftPadding

	// leftPadding-1 below reserves a column for the whale emoji's leading
	// space; on a narrow terminal leftPadding can be 0, which would make
	// that a negative Repeat count and panic.
	whaleLeadPadding := leftPadding - 1
	if whaleLeadPadding < 0 {
		whaleLeadPadding = 0
	}

	// Top border with Docker-themed color (blue)
	fmt.Printf("%s%s╔", terminal.ColorBold, terminal.ColorBrightBlue)
	for i := 0; i < innerWidth; i++ {
		fmt.Printf("═")
	}
	fmt.Printf("╗%s\n", terminal.ColorReset)

	// Middle line with Docker whale emoji
	fmt.Printf("%s%s║%s%s🐳  %s%s%s%s║%s\n",
		terminal.ColorBold, terminal.ColorBrightBlue,
		strings.Repeat(" ", whaleLeadPadding), // -1 for whale emoji space
		terminal.ColorBold, terminal.ColorBrightCyan, text, terminal.ColorReset,
		strings.Repeat(" ", rightPadding),
		terminal.ColorReset)

	// Bottom border
	fmt.Printf("%s%s╚", terminal.ColorBold, terminal.ColorBrightBlue)
	for i := 0; i < innerWidth; i++ {
		fmt.Printf("═")
	}
	fmt.Printf("╝%s\n", terminal.ColorReset)
}

func (d *Displayer) printModernSection(title string) {
	fmt.Printf("\n  %s%s▌%s %s%s%s\n",
		terminal.ColorBold, terminal.ColorBrightBlue, terminal.ColorReset,
		terminal.ColorBold, title, terminal.ColorReset)
}

func (d *Displayer) printEnhancedField(label string, value string, valueColor string, emoji string) {
	if emoji != "" {
		fmt.Printf("  %s%s%-20s%s %s%s %s%s%s\n",
			terminal.ColorBold, d.theme.Label, label+":", terminal.ColorReset,
			emoji, valueColor, value, terminal.ColorReset, terminal.ColorReset)
	} else {
		fmt.Printf("  %s%s%-20s%s %s%s%s\n",
			terminal.ColorBold, d.theme.Label, label+":", terminal.ColorReset,
			valueColor, value, terminal.ColorReset)
	}
}

func (d *Displayer) printGradientDivider() {
	fmt.Printf("%s%s%s%s\n",
		terminal.ColorBold, terminal.ColorBrightBlue,
		strings.Repeat("─", d.width), terminal.ColorReset)
}

func (d *Displayer) getStateColor(state string) string {
	state = strings.ToLower(state)
	switch state {
	case "running":
		return terminal.ColorBrightGreen
	case "paused":
		return terminal.ColorYellow
	case "exited", "dead":
		return terminal.ColorRed
	case "restarting":
		return terminal.ColorOrange
	case "created":
		return terminal.ColorCyan
	default:
		return terminal.ColorDim
	}
}

func (d *Displayer) getStateEmoji(state string) string {
	state = strings.ToLower(state)
	switch state {
	case "running":
		return "✅"
	case "paused":
		return "⏸"
	case "exited":
		return "🛑"
	case "dead":
		return "💀"
	case "restarting":
		return "🔄"
	case "created":
		return "🆕"
	default:
		return "❓"
	}
}
