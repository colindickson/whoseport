package interactive

import (
	"fmt"
	"strings"

	"github.com/bluehoodie/whoseport/internal/display/format"
	"github.com/bluehoodie/whoseport/internal/model"
	"github.com/bluehoodie/whoseport/internal/terminal"
)

// Displayer outputs ProcessInfo as interactive colorized UI
type Displayer struct {
	theme terminal.Theme
	width int
}

// NewDisplayer creates a new interactive displayer
func NewDisplayer() *Displayer {
	return &Displayer{
		theme: terminal.DefaultTheme(),
		width: terminal.Width(),
	}
}

// Display outputs the ProcessInfo as interactive UI
func (d *Displayer) Display(info *model.ProcessInfo, port int) {
	// Banner
	d.printGradientBanner(fmt.Sprintf("PORT %d ANALYSIS", port))

	// Section 1: Process Identity
	d.printModernSection("⚙️  PROCESS IDENTITY")
	d.printEnhancedField("Command", info.Command, terminal.ColorBrightGreen, "")
	d.printEnhancedField("Full Command", format.Truncate(info.FullCommand, 60), terminal.ColorLime, "")
	d.printEnhancedField("Process ID", fmt.Sprintf("%d", info.ID), terminal.ColorOrange, "")
	d.printEnhancedField("Parent PID", fmt.Sprintf("%d", info.PPid), terminal.ColorPeach, "")
	if info.ParentCommand != "" {
		d.printEnhancedField("Parent Process", info.ParentCommand, terminal.ColorLavender, "")
	}
	d.printEnhancedField("User", info.User, terminal.ColorGold, "")
	d.printEnhancedField("UID / GID", fmt.Sprintf("%d / %d", info.UID, info.GID), terminal.ColorDim, "")
	if info.ChildCount > 0 {
		d.printEnhancedField("Child Processes", fmt.Sprintf("%d", info.ChildCount), terminal.ColorMint, "")
	}

	// Section 2: Binary Information
	if info.ExePath != "" {
		d.printModernSection("📦 BINARY INFORMATION")
		d.printEnhancedField("Executable Path", info.ExePath, terminal.ColorCyan, "")
		if info.ExeSize > 0 {
			d.printEnhancedField("Binary Size", format.FormatBytes(info.ExeSize), terminal.ColorTeal, "")
		}
		d.printEnhancedField("Environment Vars", fmt.Sprintf("%d", info.EnvCount), terminal.ColorLavender, "")
		if info.WorkingDir != "" {
			d.printEnhancedField("Working Directory", info.WorkingDir, terminal.ColorSkyBlue, "")
		}
	}

	// Section 3: Process State
	d.printModernSection("📊 PROCESS STATE")
	stateEmoji := format.GetStateEmoji(info.State)
	d.printEnhancedField("State", fmt.Sprintf("%s %s", stateEmoji, info.State), format.GetStateColor(info.State, d.theme), "")
	d.printEnhancedField("Threads", fmt.Sprintf("%d", info.Threads), terminal.ColorViolet, "")
	if info.NiceValue != 0 || info.Priority != 0 {
		d.printEnhancedField("Nice / Priority", fmt.Sprintf("%d / %d", info.NiceValue, info.Priority), terminal.ColorPeach, "")
	}
	if info.StartTime != "" {
		d.printEnhancedField("Started", info.StartTime, terminal.ColorCyan, "")
	}
	if info.Uptime != "" {
		d.printEnhancedField("Uptime", info.Uptime, terminal.ColorLime, "⏱")
	}
	if info.CPUTime > 0 {
		cpuStr := fmt.Sprintf("%.2fs", info.CPUTime)
		if info.CPUPercent > 0 {
			cpuStr += fmt.Sprintf(" (%.2f%%)", info.CPUPercent)
		}
		d.printEnhancedField("CPU Time", cpuStr, terminal.ColorOrange, "")
	}

	// Section 4: Memory Usage
	d.printModernSection("💾 MEMORY USAGE")
	if info.MemoryRSS > 0 {
		d.printEnhancedField("Resident Set (RSS)", format.FormatMemory(info.MemoryRSS), terminal.ColorLime, "")
	}
	if info.MemoryVMS > 0 {
		d.printEnhancedField("Virtual Memory", format.FormatMemory(info.MemoryVMS), terminal.ColorSkyBlue, "")
	}
	if info.MemoryLimit > 0 {
		d.printEnhancedField("Memory Limit", format.FormatMemory(info.MemoryLimit), terminal.ColorDim, "")
	}

	// Section 5: I/O Statistics
	if info.IOReadBytes > 0 || info.IOWriteBytes > 0 {
		d.printModernSection("💿 I/O STATISTICS")
		if info.IOReadBytes > 0 {
			d.printEnhancedField("Read", format.FormatBytes(info.IOReadBytes), terminal.ColorBrightCyan, "📖")
			if info.IOReadSyscalls > 0 {
				d.printEnhancedField("  Read Syscalls", fmt.Sprintf("%d", info.IOReadSyscalls), terminal.ColorDim, "")
			}
		}
		if info.IOWriteBytes > 0 {
			d.printEnhancedField("Write", format.FormatBytes(info.IOWriteBytes), terminal.ColorCoral, "📝")
			if info.IOWriteSyscalls > 0 {
				d.printEnhancedField("  Write Syscalls", fmt.Sprintf("%d", info.IOWriteSyscalls), terminal.ColorDim, "")
			}
		}
	}

	// Section 6: File Descriptors
	d.printModernSection("📁 FILE DESCRIPTORS")
	if info.MaxFDs > 0 {
		fdPercent := float64(info.OpenFDs) / float64(info.MaxFDs) * 100
		fdBar := d.createProgressBar(fdPercent, 30)
		d.printEnhancedField("Open FDs", fmt.Sprintf("%d / %d (%.1f%%)", info.OpenFDs, info.MaxFDs, fdPercent), terminal.ColorYellow, "")
		fmt.Printf("  %s\n", fdBar)
	} else {
		d.printEnhancedField("Open FDs", fmt.Sprintf("%d / N/A", info.OpenFDs), terminal.ColorYellow, "")
		fmt.Printf("  %s\n", "N/A")
	}

	// Section 7: Network Information
	d.printModernSection("🌐 NETWORK")
	d.printEnhancedField("Protocol", strings.ToUpper(info.Type), terminal.ColorBrightCyan, "")
	d.printEnhancedField("Listening On", info.Name, terminal.ColorBrightGreen, "🎧")
	d.printEnhancedField("Node Type", info.Node, terminal.ColorLavender, "")
	d.printEnhancedField("File Descriptor", info.FD, terminal.ColorDim, "")
	d.printEnhancedField("Total Connections", fmt.Sprintf("%d", info.NetworkConns), terminal.ColorOrange, "")

	if len(info.TCPConns) > 0 {
		fmt.Printf("  %s%s%s▸ TCP Connections%s\n", terminal.ColorBold, terminal.ColorBrightCyan, terminal.ColorReset, terminal.ColorReset)
		for i, conn := range info.TCPConns {
			if i < 8 {
				fmt.Printf("    %s┃%s %s%s%s\n", terminal.ColorBrightBlue, terminal.ColorReset, terminal.ColorMint, conn, terminal.ColorReset)
			}
		}
		if len(info.TCPConns) > 8 {
			fmt.Printf("    %s┗━ and %d more...%s\n", terminal.ColorDim, len(info.TCPConns)-8, terminal.ColorReset)
		}
	}

	if len(info.UDPConns) > 0 {
		fmt.Printf("  %s%s%s▸ UDP Connections%s\n", terminal.ColorBold, terminal.ColorBrightCyan, terminal.ColorReset, terminal.ColorReset)
		for i, conn := range info.UDPConns {
			if i < 5 {
				fmt.Printf("    %s┃%s %s%s%s\n", terminal.ColorBrightBlue, terminal.ColorReset, terminal.ColorLavender, conn, terminal.ColorReset)
			}
		}
		if len(info.UDPConns) > 5 {
			fmt.Printf("    %s┗━ and %d more...%s\n", terminal.ColorDim, len(info.UDPConns)-5, terminal.ColorReset)
		}
	}
	d.printGradientDivider()
}

func (d *Displayer) printGradientBanner(text string) {
	textLen := format.VisualWidth(text)
	emojiWidth := 2
	spaceWidth := 1
	totalTextLen := textLen + emojiWidth + spaceWidth

	innerWidth := d.width - 2
	if innerWidth < totalTextLen {
		innerWidth = totalTextLen
	}
	leftPadding := (innerWidth - totalTextLen) / 2
	rightPadding := innerWidth - totalTextLen - leftPadding

	// Top border
	fmt.Printf("%s%s╔", terminal.ColorBold, d.theme.Primary)
	for i := 0; i < innerWidth; i++ {
		fmt.Printf("═")
	}
	fmt.Printf("╗%s\n", terminal.ColorReset)

	// Middle line
	fmt.Printf("%s%s║%s%s🔍 %s%s%s%s║%s\n",
		terminal.ColorBold, d.theme.Primary,
		strings.Repeat(" ", leftPadding),
		terminal.ColorBold, terminal.ColorBrightCyan, text, terminal.ColorReset,
		strings.Repeat(" ", rightPadding),
		terminal.ColorReset)

	// Bottom border
	fmt.Printf("%s%s╚", terminal.ColorBold, d.theme.Primary)
	for i := 0; i < innerWidth; i++ {
		fmt.Printf("═")
	}
	fmt.Printf("╝%s\n", terminal.ColorReset)
}

func (d *Displayer) printModernSection(title string) {
	fmt.Printf("\n  %s%s▌%s %s%s%s\n", terminal.ColorBold, d.theme.Primary, terminal.ColorReset, terminal.ColorBold, title, terminal.ColorReset)
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

func (d *Displayer) createProgressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100.0)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	empty := width - filled

	// Color based on percentage
	barColor := terminal.ColorLime
	if percent > 75 {
		barColor = terminal.ColorOrange
	}
	if percent > 90 {
		barColor = terminal.ColorRed
	}

	bar := fmt.Sprintf("%s[%s%s%s]%s",
		terminal.ColorDim,
		barColor, strings.Repeat("█", filled)+strings.Repeat("░", empty),
		terminal.ColorDim,
		terminal.ColorReset)

	return bar
}

func (d *Displayer) printGradientDivider() {
	fmt.Printf("%s%s%s%s\n", terminal.ColorBold, d.theme.Primary, strings.Repeat("─", d.width), terminal.ColorReset)
}
