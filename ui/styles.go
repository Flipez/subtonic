package ui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

var (
	colorFocused   = lipgloss.Color("#C4A7E7")
	colorUnfocused = lipgloss.Color("#26233A")
	colorText      = lipgloss.Color("#E0DEF4")
	colorSubtext   = lipgloss.Color("#908CAA")
	colorHighlight = lipgloss.Color("#EA9A97")
	colorPlaying   = lipgloss.Color("#9CCFD8")
	colorDimText   = lipgloss.Color("#6E6A86")
	colorError     = lipgloss.Color("#EB6F92")

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
)

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
			PaddingLeft(2),
	}
}
