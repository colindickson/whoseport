package terminal

// Color codes for terminal output (256-color palette)
const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"
	ColorDim   = "\033[2m"

	// Basic colors
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"

	// Bright colors
	ColorBrightGreen = "\033[92m"
	ColorBrightBlue  = "\033[94m"
	ColorBrightCyan  = "\033[96m"

	// 256-color palette for vibrant UI
	ColorOrange   = "\033[38;5;208m"
	ColorViolet   = "\033[38;5;141m"
	ColorSkyBlue  = "\033[38;5;117m"
	ColorLime     = "\033[38;5;154m"
	ColorGold     = "\033[38;5;220m"
	ColorCoral    = "\033[38;5;210m"
	ColorTeal     = "\033[38;5;80m"
	ColorLavender = "\033[38;5;183m"
	ColorMint     = "\033[38;5;121m"
	ColorPeach    = "\033[38;5;216m"
)

// Theme defines the color scheme for the UI
type Theme struct {
	Primary       string
	Label         string
	StateRunning  string
	StateSleeping string
	StateZombie   string
	StateStopped  string
	StateDefault  string
}

// DefaultTheme returns the default color theme
func DefaultTheme() Theme {
	return Theme{
		Primary:       ColorCyan,
		Label:         ColorYellow,
		StateRunning:  ColorGreen,
		StateSleeping: ColorCyan,
		StateZombie:   ColorRed,
		StateStopped:  ColorYellow,
		StateDefault:  ColorWhite,
	}
}
