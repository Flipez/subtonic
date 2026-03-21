package ui

import (
	"image/color"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// Color palette — set by InitTheme.
var (
	colorFocused   color.Color
	colorUnfocused color.Color
	colorText      color.Color
	colorSubtext   color.Color
	colorHighlight color.Color
	colorPlaying   color.Color
	colorDimText   color.Color
	colorError     color.Color
)

// Styles — set by InitTheme.
var (
	CardStyle      lipgloss.Style
	TitleBarStyle  lipgloss.Style
	NavTabStyle    lipgloss.Style
	NavTabActiveStyle lipgloss.Style
	BreadcrumbStyle   lipgloss.Style
	TitleStyle        lipgloss.Style
	NowPlayingStyle   lipgloss.Style
	SubtextStyle      lipgloss.Style
	PlayerBarStyle    lipgloss.Style
	PlayerIconStyle   lipgloss.Style
	PlayerAlbumStyle  lipgloss.Style
	PlayerTimeStyle   lipgloss.Style
	PlayerMetaStyle   lipgloss.Style
	PlayerHelpKeyStyle lipgloss.Style
	PlayerHelpSepStyle lipgloss.Style
	ToastSuccessStyle  lipgloss.Style
	ToastErrorStyle    lipgloss.Style
	ToastInfoStyle     lipgloss.Style
	HelpSectionStyle   lipgloss.Style
	HelpKeyStyle       lipgloss.Style
	HelpDescStyle      lipgloss.Style
)

// InitTheme sets the color palette and all styles based on the terminal
// background. isDark should come from lipgloss.HasDarkBackground.
// Dark: Rosé Pine. Light: Rosé Pine Dawn.
func InitTheme(isDark bool) {
	if isDark {
		colorFocused   = lipgloss.Color("#C4A7E7")
		colorUnfocused = lipgloss.Color("#26233A")
		colorText      = lipgloss.Color("#E0DEF4")
		colorSubtext   = lipgloss.Color("#908CAA")
		colorHighlight = lipgloss.Color("#EA9A97")
		colorPlaying   = lipgloss.Color("#9CCFD8")
		colorDimText   = lipgloss.Color("#6E6A86")
		colorError     = lipgloss.Color("#EB6F92")
	} else {
		// Rosé Pine Dawn
		colorFocused   = lipgloss.Color("#907aa9")
		colorUnfocused = lipgloss.Color("#dfdad9")
		colorText      = lipgloss.Color("#575279")
		colorSubtext   = lipgloss.Color("#797593")
		colorHighlight = lipgloss.Color("#d7827e")
		colorPlaying   = lipgloss.Color("#56949f")
		colorDimText   = lipgloss.Color("#9893a5")
		colorError     = lipgloss.Color("#b4637a")
	}

	CardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorUnfocused).
		Padding(0, 1)

	TitleBarStyle = lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true)

	NavTabStyle = lipgloss.NewStyle().
		Foreground(colorSubtext).
		Padding(0, 1)

	NavTabActiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(colorFocused).
		Bold(true).
		Padding(0, 1)

	BreadcrumbStyle = lipgloss.NewStyle().
		Foreground(colorSubtext)

	TitleStyle = lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true).
		MarginBottom(1)

	NowPlayingStyle = lipgloss.NewStyle().
		Foreground(colorPlaying).
		Bold(true)

	SubtextStyle = lipgloss.NewStyle().
		Foreground(colorSubtext)

	PlayerBarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorUnfocused).
		Padding(0, 1)

	PlayerIconStyle = lipgloss.NewStyle().
		Foreground(colorPlaying).
		Bold(true)

	PlayerAlbumStyle = lipgloss.NewStyle().
		Foreground(colorHighlight)

	PlayerTimeStyle = lipgloss.NewStyle().
		Foreground(colorText)

	PlayerMetaStyle = lipgloss.NewStyle().
		Foreground(colorSubtext)

	PlayerHelpKeyStyle = lipgloss.NewStyle().
		Foreground(colorSubtext)

	PlayerHelpSepStyle = lipgloss.NewStyle().
		Foreground(colorDimText)

	ToastSuccessStyle = lipgloss.NewStyle().
		Foreground(colorPlaying)

	ToastErrorStyle = lipgloss.NewStyle().
		Foreground(colorError).
		Bold(true)

	ToastInfoStyle = lipgloss.NewStyle().
		Foreground(colorSubtext)

	HelpSectionStyle = lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(colorFocused)

	HelpDescStyle = lipgloss.NewStyle().
		Foreground(colorText)
}

func SongTableStyles() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().
			Foreground(colorSubtext).
			Bold(true).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true).
			Padding(0, 1),
	}
}
