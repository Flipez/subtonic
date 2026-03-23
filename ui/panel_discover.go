package ui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/config"
)

const maxVisibleCards = 5

// homeSections returns sections for the Home tab (recently/most played).
func (m *Model) homeSections() []discoverSec {
	var secs []discoverSec
	if len(m.recentPlaylists) > 0 {
		secs = append(secs, discoverSec{label: "Last Played Playlists", kind: secRecentPlaylists, count: len(m.recentPlaylists)})
	}
	if len(m.discoverRecent) > 0 {
		secs = append(secs, discoverSec{label: "Recently Played", kind: secRecent, count: len(m.discoverRecent)})
	}
	if len(m.discoverFrequent) > 0 {
		secs = append(secs, discoverSec{label: "Most Played", kind: secFrequent, count: len(m.discoverFrequent)})
	}
	return secs
}

// currentSections returns the sections for whichever grid view is active.
func (m *Model) currentSections() []discoverSec {
	if m.viewType == ViewHome {
		return m.homeSections()
	}
	switch m.discoverSubTab {
	case SubTabCharts:
		return m.chartsSections()
	case SubTabLibrary:
		return m.librarySections()
	default:
		return m.forYouSections()
	}
}

func (m *Model) forYouSections() []discoverSec {
	var secs []discoverSec
	if len(m.lbRecommended) > 0 {
		secs = append(secs, discoverSec{label: "Recommended for You", kind: secLBRecommended, count: len(m.lbRecommended)})
	}
	if len(m.lbPopular) > 0 {
		label := "Popular by Artist"
		if m.lbPopularArtist != "" {
			label = "Popular by " + m.lbPopularArtist
		}
		secs = append(secs, discoverSec{label: label, kind: secLBPopular, count: len(m.lbPopular)})
	}
	for i, pl := range m.lbCreatedForPlaylists {
		secs = append(secs, discoverSec{label: pl.Name, kind: secLBCreatedFor, count: len(pl.Tracks), index: i})
	}
	return secs
}

func (m *Model) chartsSections() []discoverSec {
	var secs []discoverSec
	if len(m.lbTrending) > 0 {
		secs = append(secs, discoverSec{label: "Trending", kind: secLBTrending, count: len(m.lbTrending)})
	}
	if len(m.lbFreshReleases) > 0 {
		secs = append(secs, discoverSec{label: "Fresh Releases", kind: secLBFreshReleases, count: len(m.lbFreshReleases)})
	}
	return secs
}

func (m *Model) librarySections() []discoverSec {
	var secs []discoverSec
	if len(m.discoverNewest) > 0 {
		secs = append(secs, discoverSec{label: "Recently Added", kind: secNewest, count: len(m.discoverNewest)})
	}
	return secs
}

type secKind int

const (
	secRecent secKind = iota
	secNewest
	secFrequent
	secLBTrending
	secLBFreshReleases
	secLBPopular
	secRecentPlaylists
	secLBCreatedFor
	secLBRecommended
)

type discoverSec struct {
	label string
	kind  secKind
	count int
	index int // used by secLBCreatedFor to index into lbCreatedForPlaylists
}

func (m *Model) renderHome(contentW, contentH int) string {
	return m.renderSectionGrid(m.homeSections(), contentW, contentH)
}

func (m *Model) renderDiscover(contentW, contentH int) string {
	return m.renderSectionGrid(m.currentSections(), contentW, contentH)
}

func (m *Model) renderSectionGrid(secs []discoverSec, contentW, contentH int) string {
	if len(secs) == 0 {
		return ""
	}

	// Clamp section/item
	if m.discoverSection >= len(secs) {
		m.discoverSection = len(secs) - 1
	}
	sec := secs[m.discoverSection]
	if m.discoverItem >= sec.count {
		m.discoverItem = sec.count - 1
	}
	if m.discoverItem < 0 {
		m.discoverItem = 0
	}

	// Render each section and track heights
	type renderedSection struct {
		content string
		height  int
	}
	var rendered []renderedSection

	for i, s := range secs {
		isFocused := i == m.discoverSection

		labelStyle := SubtextStyle
		if isFocused {
			labelStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
		}
		labelText := labelStyle.Render(s.label)

		// Show scroll arrows for sections with more items than visible
		if s.count > maxVisibleCards {
			arrowStyle := SubtextStyle
			if isFocused {
				arrowStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
			}
			arrow := arrowStyle.Render(IconChevronRight)
			padding := contentW - lipgloss.Width(labelText) - lipgloss.Width(arrow) - 2
			if padding < 1 {
				padding = 1
			}
			labelText = labelText + strings.Repeat(" ", padding) + arrow
		}

		label := labelText

		var body string
		switch s.kind {
		case secRecentPlaylists:
			body = m.renderPlaylistRow(m.recentPlaylists, contentW, isFocused)
		case secRecent:
			body = m.renderAlbumRow(m.discoverRecent, contentW, isFocused)
		case secNewest:
			body = m.renderAlbumRow(m.discoverNewest, contentW, isFocused)
		case secFrequent:
			body = m.renderAlbumRow(m.discoverFrequent, contentW, isFocused)
		case secLBTrending:
			body = m.renderTrackRow(m.lbTrending, contentW, isFocused)
		case secLBFreshReleases:
			body = m.renderReleaseRow(m.lbFreshReleases, contentW, isFocused)
		case secLBPopular:
			body = m.renderTrackRow(m.lbPopular, contentW, isFocused)
		case secLBCreatedFor:
			if s.index < len(m.lbCreatedForPlaylists) {
				body = m.renderTrackRow(m.lbCreatedForPlaylists[s.index].Tracks, contentW, isFocused)
			}
		case secLBRecommended:
			body = m.renderTrackRow(m.lbRecommended, contentW, isFocused)
		}

		block := lipgloss.JoinVertical(lipgloss.Left, label, body)
		rendered = append(rendered, renderedSection{content: block, height: lipgloss.Height(block)})
	}

	// Scroll: ensure focused section is visible
	// Find the Y offset of the focused section and scroll so it fits
	scrollOffset := 0
	focusedY := 0
	focusedH := 0
	totalH := 0
	for i, rs := range rendered {
		if i == m.discoverSection {
			focusedY = totalH
			focusedH = rs.height
		}
		totalH += rs.height
	}

	if totalH > contentH {
		// If focused section bottom exceeds viewport, scroll down
		if focusedY+focusedH > scrollOffset+contentH {
			scrollOffset = focusedY + focusedH - contentH
		}
		// If focused section top is above viewport, scroll up
		if focusedY < scrollOffset {
			scrollOffset = focusedY
		}
	}

	// Build output by joining all sections then slicing lines
	var allParts []string
	for _, rs := range rendered {
		allParts = append(allParts, rs.content)
	}
	full := lipgloss.JoinVertical(lipgloss.Left, allParts...)
	lines := splitLines(full)

	// Apply scroll offset
	if scrollOffset > len(lines) {
		scrollOffset = len(lines)
	}
	end := scrollOffset + contentH
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scrollOffset:end]

	// Pad to fill content height
	for len(visible) < contentH {
		visible = append(visible, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, visible...)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func (m *Model) renderAlbumRow(albums []api.Album, contentW int, focused bool) string {
	if len(albums) == 0 {
		return ""
	}

	n := len(albums)
	maxVisible := maxVisibleCards
	if n < maxVisible {
		maxVisible = n
	}
	cardW := contentW / maxVisible
	if cardW < 14 {
		cardW = 14
	}

	// Scroll to keep selected item visible
	offset := 0
	if focused && m.discoverItem >= maxVisible {
		offset = m.discoverItem - maxVisible + 1
	}
	end := offset + maxVisible
	if end > n {
		end = n
	}
	visibleAlbums := albums[offset:end]

	visibleCount := len(visibleAlbums)
	var cards []string
	for i, a := range visibleAlbums {
		actualIdx := offset + i
		selected := focused && actualIdx == m.discoverItem

		w := cardW
		if i == visibleCount-1 {
			w = contentW - cardW*(visibleCount-1)
		}
		iw := w - 2 // content area inside border

		name := truncateStr(a.Name, iw)
		var subtitleParts []string
		if a.Year > 0 {
			subtitleParts = append(subtitleParts, fmt.Sprintf("%d", a.Year))
		}
		if a.Artist != "" {
			subtitleParts = append(subtitleParts, a.Artist)
		}
		subtitle := truncateStr(strings.Join(subtitleParts, " · "), iw)

		var borderColor color.Color = colorUnfocused
		nameStyle := lipgloss.NewStyle().Foreground(colorText)

		if selected {
			borderColor = colorFocused
			nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
		}

		content := nameStyle.Render(name) + "\n" +
			SubtextStyle.Render(subtitle)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Width(w).
			Height(2).
			Render(content)

		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (m *Model) renderPlaylistRow(playlists []config.RecentPlaylist, contentW int, focused bool) string {
	if len(playlists) == 0 {
		return ""
	}

	n := len(playlists)
	maxVisible := maxVisibleCards
	if n < maxVisible {
		maxVisible = n
	}
	cardW := contentW / maxVisible
	if cardW < 14 {
		cardW = 14
	}

	offset := 0
	if focused && m.discoverItem >= maxVisible {
		offset = m.discoverItem - maxVisible + 1
	}
	end := offset + maxVisible
	if end > n {
		end = n
	}
	visible := playlists[offset:end]

	var cards []string
	for i, pl := range visible {
		actualIdx := offset + i
		selected := focused && actualIdx == m.discoverItem

		w := cardW
		if i == len(visible)-1 {
			w = contentW - cardW*(len(visible)-1)
		}
		iw := w - 2

		name := truncateStr(pl.Name, iw)
		subtitle := truncateStr(fmt.Sprintf("%d tracks", pl.SongCount), iw)

		var borderColor color.Color = colorUnfocused
		nameStyle := lipgloss.NewStyle().Foreground(colorText)
		if selected {
			borderColor = colorFocused
			nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
		}

		content := nameStyle.Render(name) + "\n" +
			SubtextStyle.Render(subtitle)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Width(w).
			Height(2).
			Render(content)

		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (m *Model) renderTrackRow(tracks []DiscoverTrack, contentW int, focused bool) string {
	if len(tracks) == 0 {
		return ""
	}

	n := len(tracks)
	maxVisible := maxVisibleCards
	if n < maxVisible {
		maxVisible = n
	}
	cardW := contentW / maxVisible
	if cardW < 14 {
		cardW = 14
	}
	maxCards := maxVisible
	if maxCards < 1 {
		maxCards = 1
	}

	// Scroll to keep selected item visible
	offset := 0
	if focused && m.discoverItem >= maxCards {
		offset = m.discoverItem - maxCards + 1
	}
	end := offset + maxCards
	if end > n {
		end = n
	}
	visible := tracks[offset:end]

	visibleCount := len(visible)
	var cards []string
	for i, dt := range visible {
		actualIdx := offset + i
		selected := focused && actualIdx == m.discoverItem

		w := cardW
		if i == visibleCount-1 {
			w = contentW - cardW*(visibleCount-1)
		}
		iw := w - 2 // content area inside border

		var borderColor color.Color
		var nameStyle lipgloss.Style
		var artistStyle lipgloss.Style
		var indicator string

		if dt.Available {
			indicator = lipgloss.NewStyle().Foreground(colorPlaying).Render(IconCheck)
			if selected {
				borderColor = colorFocused
				nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
			} else {
				borderColor = colorUnfocused
				nameStyle = lipgloss.NewStyle().Foreground(colorText)
			}
			artistStyle = SubtextStyle
		} else {
			indicator = lipgloss.NewStyle().Foreground(colorDimText).Render(IconClose)
			if selected {
				borderColor = colorDimText
			} else {
				borderColor = colorUnfocused
			}
			nameStyle = lipgloss.NewStyle().Foreground(colorDimText)
			artistStyle = lipgloss.NewStyle().Foreground(colorDimText)
		}

		prefixW := lipgloss.Width(indicator) + 1 // indicator + space
		title := truncateStr(dt.Title, iw-prefixW)
		artist := truncateStr(dt.Artist, iw)

		content := indicator + " " + nameStyle.Render(title) + "\n" +
			artistStyle.Render(artist)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Width(w).
			Height(2).
			Render(content)

		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (m *Model) renderReleaseRow(releases []DiscoverRelease, contentW int, focused bool) string {
	if len(releases) == 0 {
		return ""
	}

	n := len(releases)
	maxVisible := maxVisibleCards
	if n < maxVisible {
		maxVisible = n
	}
	cardW := contentW / maxVisible
	if cardW < 14 {
		cardW = 14
	}
	maxCards := maxVisible
	if maxCards < 1 {
		maxCards = 1
	}

	offset := 0
	if focused && m.discoverItem >= maxCards {
		offset = m.discoverItem - maxCards + 1
	}
	end := offset + maxCards
	if end > n {
		end = n
	}
	visible := releases[offset:end]

	visibleCount := len(visible)
	var cards []string
	for i, dr := range visible {
		actualIdx := offset + i
		selected := focused && actualIdx == m.discoverItem

		w := cardW
		if i == visibleCount-1 {
			w = contentW - cardW*(visibleCount-1)
		}
		iw := w - 2 // content area inside border

		var borderColor color.Color
		var nameStyle lipgloss.Style
		var artistStyle lipgloss.Style
		var indicator string

		if dr.Available {
			indicator = lipgloss.NewStyle().Foreground(colorPlaying).Render(IconCheck)
			if selected {
				borderColor = colorFocused
				nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
			} else {
				borderColor = colorUnfocused
				nameStyle = lipgloss.NewStyle().Foreground(colorText)
			}
			artistStyle = SubtextStyle
		} else {
			indicator = lipgloss.NewStyle().Foreground(colorDimText).Render(IconClose)
			if selected {
				borderColor = colorDimText
			} else {
				borderColor = colorUnfocused
			}
			nameStyle = lipgloss.NewStyle().Foreground(colorDimText)
			artistStyle = lipgloss.NewStyle().Foreground(colorDimText)
		}

		prefixW := lipgloss.Width(indicator) + 1 // indicator + space
		title := truncateStr(dr.Title, iw-prefixW)
		artist := truncateStr(dr.Artist, iw)
		date := truncateStr(dr.Date, iw)

		content := indicator + " " + nameStyle.Render(title) + "\n" +
			artistStyle.Render(artist) + "\n" +
			lipgloss.NewStyle().Foreground(colorDimText).Render(date)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Width(w).
			Height(3).
			Render(content)

		cards = append(cards, card)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func renderChip(label string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(colorFocused).
			Bold(true).
			Padding(0, 1).
			Render(label)
	}
	return lipgloss.NewStyle().
		Foreground(colorSubtext).
		Padding(0, 1).
		Render(label)
}

func truncateStr(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxW-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func (m *Model) discoverMove(dSection, dItem int) (tea.Model, tea.Cmd) {
	secs := m.currentSections()
	if len(secs) == 0 {
		return m, nil
	}
	m.discoverSection += dSection
	if m.discoverSection < 0 {
		m.discoverSection = 0
	}
	if m.discoverSection >= len(secs) {
		m.discoverSection = len(secs) - 1
	}
	sec := secs[m.discoverSection]
	m.discoverItem += dItem
	if m.discoverItem < 0 {
		m.discoverItem = 0
	}
	if m.discoverItem >= sec.count {
		m.discoverItem = sec.count - 1
	}
	return m, nil
}

func (m *Model) handleDiscoverEnter() (tea.Model, tea.Cmd) {
	secs := m.currentSections()
	if m.discoverSection >= len(secs) {
		return m, nil
	}
	sec := secs[m.discoverSection]

	switch sec.kind {
	case secRecentPlaylists:
		if m.discoverItem < len(m.recentPlaylists) {
			pl := m.recentPlaylists[m.discoverItem]
			m.currentPlaylistID = pl.ID
			m.pushNav(pl.Name)
			return m.loadWithSpinner(loadPlaylistCmd(m.api, pl.ID))
		}

	case secRecent:
		if m.discoverItem < len(m.discoverRecent) {
			album := m.discoverRecent[m.discoverItem]
			m.pushNav(album.Name)
			return m.loadWithSpinner(loadAlbumCmd(m.api, album.ID))
		}

	case secNewest:
		if m.discoverItem < len(m.discoverNewest) {
			album := m.discoverNewest[m.discoverItem]
			m.pushNav(album.Name)
			return m.loadWithSpinner(loadAlbumCmd(m.api, album.ID))
		}

	case secFrequent:
		if m.discoverItem < len(m.discoverFrequent) {
			album := m.discoverFrequent[m.discoverItem]
			m.pushNav(album.Name)
			return m.loadWithSpinner(loadAlbumCmd(m.api, album.ID))
		}

	case secLBTrending, secLBPopular, secLBCreatedFor, secLBRecommended:
		var tracks []DiscoverTrack
		switch sec.kind {
		case secLBTrending:
			tracks = m.lbTrending
		case secLBPopular:
			tracks = m.lbPopular
		case secLBCreatedFor:
			if sec.index < len(m.lbCreatedForPlaylists) {
				tracks = m.lbCreatedForPlaylists[sec.index].Tracks
			}
		case secLBRecommended:
			tracks = m.lbRecommended
		}
		if m.discoverItem >= len(tracks) {
			return m, nil
		}
		dt := tracks[m.discoverItem]
		if !dt.Available {
			return m, m.showToast("Not in library", ToastInfo, toastShort)
		}
		// Queue all available tracks from section and play selected
		var songs []api.Song
		var startIdx int
		for _, t := range tracks {
			if t.Available {
				if t.Song.ID == dt.Song.ID {
					startIdx = len(songs)
				}
				songs = append(songs, t.Song)
			}
		}
		if len(songs) == 0 {
			return m, nil
		}
		m.pl.Queue().Set(songs, startIdx)
		song := dt.Song
		streamURL := m.api.StreamURL(song.ID)
		return m, func() tea.Msg {
			if err := m.pl.Play(song, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			go m.api.Scrobble(song.ID) //nolint:errcheck
			return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", song.Title, song.Artist), Level: ToastSuccess}
		}

	case secLBFreshReleases:
		if m.discoverItem >= len(m.lbFreshReleases) {
			return m, nil
		}
		dr := m.lbFreshReleases[m.discoverItem]
		if !dr.Available {
			return m, m.showToast("Not in library", ToastInfo, toastShort)
		}
		m.pushNav(dr.Album.Name)
		return m.loadWithSpinner(loadAlbumCmd(m.api, dr.Album.ID))
	}

	return m, nil
}
