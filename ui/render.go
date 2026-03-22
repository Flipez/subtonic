package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m *Model) renderInfoPopup() string {
	maxW := 76
	if m.width-8 < maxW {
		maxW = m.width - 8
	}
	innerW := maxW - 6 // border (2) + padding left+right (4)
	if innerW < 20 {
		innerW = 20
	}
	maxLines := m.height - 10
	if maxLines < 5 {
		maxLines = 5
	}

	title := lipgloss.NewStyle().Bold(true).Render(m.infoTitle)
	var body string
	if m.infoLoading {
		body = m.spinner.View() + " Loading…"
	} else {
		wrapped := wordWrap(m.infoContent, innerW)
		lines := strings.Split(wrapped, "\n")
		truncated := false
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			truncated = true
		}
		body = strings.Join(lines, "\n")
		if truncated {
			body += "\n..."
		}
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", body)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFocused).
		Padding(1, 2).
		Render(inner)
}

func (m *Model) renderToastPopup() string {
	if len(m.toasts) == 0 {
		return ""
	}
	t := m.toasts[0]
	var borderStyle lipgloss.Style
	var textStyle lipgloss.Style
	switch t.level {
	case ToastSuccess:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPlaying).Padding(0, 1)
		textStyle = ToastSuccessStyle
	case ToastError:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorError).Padding(0, 1)
		textStyle = ToastErrorStyle
	default:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorFocused).Padding(0, 1)
		textStyle = ToastInfoStyle
	}
	return borderStyle.Render(textStyle.Render(t.text))
}

func (m *Model) renderNavBar(contentW int) string {
	title := TitleBarStyle.Render("\uf000 subtonic")

	tabs := []struct {
		label string
		tab   Tab
	}{
		{"1 Home", TabHome},
		{"2 Discover", TabDiscover},
		{"3 Browse", TabBrowse},
		{"4 Playlists", TabPlaylists},
		{"5 Podcasts", TabPodcasts},
		{"6 Search", TabSearch},
	}

	parts := make([]string, len(tabs))
	for i, t := range tabs {
		if t.tab == m.activeTab {
			parts[i] = NavTabActiveStyle.Render(t.label)
		} else {
			parts[i] = NavTabStyle.Render(t.label)
		}
	}
	tabBar := strings.Join(parts, " ")

	gap := contentW - lipgloss.Width(title) - lipgloss.Width(tabBar)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + tabBar
}

func (m *Model) renderBreadcrumb() string {
	tabNames := map[Tab]string{
		TabHome:      "Home",
		TabDiscover:  "Discover",
		TabBrowse:    "Browse",
		TabPlaylists: "Playlists",
		TabPodcasts:  "Podcasts",
		TabSearch:    "Search",
	}

	parts := []string{tabNames[m.activeTab]}
	for _, level := range m.navStack {
		parts = append(parts, level.Label)
	}
	if m.viewType == ViewQueue {
		parts = append(parts, "Queue")
	}
	if m.viewType == ViewPlaylistPicker {
		parts = append(parts, "Add to Playlist")
	}

	return BreadcrumbStyle.Render(strings.Join(parts, " › "))
}

func (m *Model) renderRowCounter() string {
	if m.viewType == ViewDiscover || m.viewType == ViewHome {
		return ""
	}
	totalRows := len(m.table.Rows())
	if totalRows == 0 {
		return ""
	}
	cursor := m.table.Cursor() + 1
	counter := fmt.Sprintf("%d of %d", cursor, totalRows)

	visibleRows := m.table.Height() - 1
	if totalRows <= visibleRows {
		return PlayerMetaStyle.Render(counter + " ")
	}

	// Mini scrollbar
	barWidth := 10
	pct := float64(cursor-1) / float64(totalRows-1)
	thumbPos := int(pct * float64(barWidth-1))

	trackStyle := lipgloss.NewStyle().Foreground(colorDimText)
	thumbStyle := lipgloss.NewStyle().Foreground(colorFocused)

	var bar string
	for i := 0; i < barWidth; i++ {
		if i == thumbPos {
			bar += thumbStyle.Render("█")
		} else {
			bar += trackStyle.Render("░")
		}
	}

	return PlayerMetaStyle.Render(counter) + " " + bar + " "
}

func (m *Model) renderHelp() string {
	sep := PlayerHelpSepStyle.Render(" · ")
	var keys []string
	switch m.viewType {
	case ViewQueue:
		keys = []string{
			"enter play",
			"d remove",
			"J/K move",
			"esc close",
		}
	case ViewPlaylistPicker:
		keys = []string{
			"enter select",
			"n new",
			"esc cancel",
		}
	case ViewSonosPicker:
		keys = []string{
			"enter connect",
			"esc cancel",
		}
	default:
		switch m.viewType {
		case ViewArtists, ViewAlbums, ViewSongs, ViewPlaylists:
			keys = append(keys, "p shuffle play")
		}
		if m.viewType == ViewPlaylists && m.activeTab == TabPlaylists && len(m.navStack) == 0 {
			keys = append(keys, "n new", "d delete")
		}
		if m.viewType == ViewSongs && m.currentPlaylistID != "" {
			keys = append(keys, "d remove")
		}
		if m.viewType == ViewSongs || m.viewType == ViewSearchResults {
			keys = append(keys, "a add to playlist", "e play next")
		}
		keys = append(keys, "x actions", "? help")
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = PlayerHelpKeyStyle.Render(k)
	}
	left := " " + strings.Join(parts, sep)
	github := PlayerHelpKeyStyle.Hyperlink("https://github.com/Flipez/subtonic").Render("\uf09b")
	right := github + sep + PlayerHelpKeyStyle.Hyperlink("https://shmbrt.de").Render("by shmbrt") + " "
	return padLine(left, right, m.width)
}

func (m *Model) renderHelpPopup() string {
	type entry struct{ key, desc string }
	type section struct {
		title   string
		entries []entry
	}

	sections := []section{
		{
			"Playback",
			[]entry{
				{"space", "play / pause"},
				{"> or .", "next track"},
				{"< or ,", "previous track"},
				{"= / +", "volume +1 / +5"},
				{"- / _", "volume -1 / -5"},
				{"s", "toggle shuffle"},
				{"r", "cycle repeat"},
				{"o", "toggle Sonos output"},
			},
		},
		{
			"Navigation",
			[]entry{
				{"1 – 6", "switch tab"},
				{"/", "filter / search"},
				{"esc", "go back"},
				{"Q", "open queue"},
				{"x", "quick actions"},
				{"R", "refresh"},
				{"?", "close this help"},
				{"q", "quit"},
			},
		},
	}

	// View-specific section
	var viewTitle string
	var viewEntries []entry
	switch m.viewType {
	case ViewSongs:
		viewTitle = "Songs"
		viewEntries = []entry{
			{"enter", "play from here"},
			{"p", "shuffle play all"},
			{"e", "play next"},
			{"S", "star / unstar"},
			{"a", "add to playlist"},
		}
		if m.currentPlaylistID != "" {
			viewEntries = append(viewEntries, entry{"d / del", "remove from playlist"})
		}
	case ViewQueue:
		viewTitle = "Queue"
		viewEntries = []entry{
			{"d / del", "remove from queue"},
			{"J", "move song down"},
			{"K", "move song up"},
		}
	case ViewHome:
		viewTitle = "Home"
		viewEntries = []entry{
			{"enter", "open album"},
			{"↑↓←→ / hjkl", "navigate grid"},
		}
	case ViewBrowse:
		viewTitle = "Browse"
		viewEntries = []entry{
			{"enter", "open category"},
		}
	case ViewArtists:
		viewTitle = "Artists"
		viewEntries = []entry{
			{"enter", "browse albums"},
			{"p", "shuffle play all"},
			{"i", "artist info"},
		}
	case ViewAlbums:
		viewTitle = "Albums"
		viewEntries = []entry{
			{"enter", "browse songs"},
			{"p", "shuffle play all"},
			{"i", "album info"},
		}
	case ViewPlaylists:
		viewTitle = "Playlists"
		viewEntries = []entry{
			{"enter", "open playlist"},
			{"p", "shuffle play"},
			{"n", "new playlist"},
			{"d / del", "delete playlist"},
		}
	case ViewSearchResults:
		viewTitle = "Search Results"
		viewEntries = []entry{
			{"enter", "open / play"},
			{"S", "star / unstar"},
			{"a", "add to playlist"},
			{"e", "play next"},
		}
	case ViewGenres:
		viewTitle = "Genres"
		viewEntries = []entry{
			{"enter", "browse genre songs"},
		}
	case ViewDiscover:
		viewTitle = "Discover"
		viewEntries = []entry{
			{"enter", "explore section"},
			{"↑↓←→ / hjkl", "navigate grid"},
		}
	case ViewPodcasts:
		viewTitle = "Podcasts"
		viewEntries = []entry{
			{"enter", "open podcast"},
		}
	case ViewEpisodes:
		viewTitle = "Episodes"
		viewEntries = []entry{
			{"enter", "play episode"},
		}
	}
	if len(viewEntries) > 0 {
		sections = append(sections, section{viewTitle, viewEntries})
	}

	const keyColW = 14
	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, HelpSectionStyle.Render(sec.title))
		for _, e := range sec.entries {
			key := HelpKeyStyle.Width(keyColW).Render(e.key)
			desc := HelpDescStyle.Render(e.desc)
			lines = append(lines, "  "+key+desc)
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFocused).
		Padding(1, 2).
		Render(content)
}

func (m *Model) renderQuickActionsPopup() string {
	const labelW = 16
	var lines []string
	lines = append(lines, HelpSectionStyle.Render("Quick Actions"))
	lines = append(lines, "")
	for i, opt := range discoverOptions {
		selected := i == m.quickActionsIdx
		var prefix string
		var label string
		if selected {
			prefix = HelpKeyStyle.Render("> ")
			label = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true).Width(labelW).Render(opt.Label)
		} else {
			prefix = "  "
			label = HelpKeyStyle.Width(labelW).Render(opt.Label)
		}
		desc := HelpDescStyle.Render(opt.Description)
		lines = append(lines, prefix+label+"  "+desc)
	}
	lines = append(lines, "")
	lines = append(lines, PlayerHelpSepStyle.Render("↑↓ navigate · enter select · esc close"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFocused).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}
