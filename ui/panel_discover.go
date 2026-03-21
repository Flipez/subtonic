package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
)

const maxVisibleCards = 5

// discoverSections returns the list of active sections with their labels and item counts.
func (m *Model) discoverSections() []discoverSec {
	var secs []discoverSec
	secs = append(secs, discoverSec{label: "Quick Actions", kind: secQuickActions, count: len(discoverOptions)})
	if len(m.lbRecommended) > 0 {
		secs = append(secs, discoverSec{label: "Recommended for You (ListenBrainz)", kind: secLBRecommended, count: len(m.lbRecommended)})
	}
	if len(m.discoverRecent) > 0 {
		secs = append(secs, discoverSec{label: "Recently Played", kind: secRecent, count: len(m.discoverRecent)})
	}
	if len(m.discoverFrequent) > 0 {
		secs = append(secs, discoverSec{label: "Most Played", kind: secFrequent, count: len(m.discoverFrequent)})
	}
	if len(m.lbTrending) > 0 {
		secs = append(secs, discoverSec{label: "Trending (ListenBrainz)", kind: secLBTrending, count: len(m.lbTrending)})
	}
	if len(m.lbFreshReleases) > 0 {
		secs = append(secs, discoverSec{label: "Fresh Releases (ListenBrainz)", kind: secLBFreshReleases, count: len(m.lbFreshReleases)})
	}
	if len(m.lbPopular) > 0 {
		label := "Popular by Artist (ListenBrainz)"
		if m.lbPopularArtist != "" {
			label = "Popular by " + m.lbPopularArtist + " (ListenBrainz)"
		}
		secs = append(secs, discoverSec{label: label, kind: secLBPopular, count: len(m.lbPopular)})
	}
	if len(m.lbDailyJams) > 0 {
		label := "Daily Jams (ListenBrainz)"
		if m.lbDailyJamsName != "" {
			label = m.lbDailyJamsName
		}
		secs = append(secs, discoverSec{label: label, kind: secLBDailyJams, count: len(m.lbDailyJams)})
	}
	if len(m.lbWeekly) > 0 {
		label := "Weekly Exploration (ListenBrainz)"
		if m.lbWeeklyName != "" {
			label = m.lbWeeklyName
		}
		secs = append(secs, discoverSec{label: label, kind: secLBWeekly, count: len(m.lbWeekly)})
	}
	if len(m.discoverNewest) > 0 {
		secs = append(secs, discoverSec{label: "Recently Added", kind: secNewest, count: len(m.discoverNewest)})
	}
	return secs
}

type secKind int

const (
	secQuickActions secKind = iota
	secRecent
	secNewest
	secFrequent
	secLBTrending
	secLBFreshReleases
	secLBPopular
	secLBDailyJams
	secLBWeekly
	secLBRecommended
)

type discoverSec struct {
	label string
	kind  secKind
	count int
}

func (m *Model) renderDiscover(contentW, contentH int) string {
	secs := m.discoverSections()
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
		if s.kind != secQuickActions && s.count > maxVisibleCards {
			arrowStyle := SubtextStyle
			if isFocused {
				arrowStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
			}
			arrow := arrowStyle.Render("▶")
			padding := contentW - lipgloss.Width(labelText) - lipgloss.Width(arrow) - 2
			if padding < 1 {
				padding = 1
			}
			labelText = labelText + strings.Repeat(" ", padding) + arrow
		}

		label := labelText

		var body string
		switch s.kind {
		case secQuickActions:
			body = m.renderQuickActions(contentW, isFocused)
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
		case secLBDailyJams:
			body = m.renderTrackRow(m.lbDailyJams, contentW, isFocused)
		case secLBWeekly:
			body = m.renderTrackRow(m.lbWeekly, contentW, isFocused)
		case secLBRecommended:
			body = m.renderTrackRow(m.lbRecommended, contentW, isFocused)
		}

		var block string
		if s.kind == secQuickActions {
			// Label and chips on same line
			block = label + "  " + body
		} else {
			block = lipgloss.JoinVertical(lipgloss.Left, label, body)
		}
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

func (m *Model) renderQuickActions(contentW int, focused bool) string {
	var items []string
	for i, opt := range discoverOptions {
		selected := focused && i == m.discoverItem
		items = append(items, renderChip(opt.Label, selected))
	}
	return " " + lipgloss.JoinHorizontal(lipgloss.Center, items...)
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
		artist := truncateStr(a.Artist, iw)
		var detailParts []string
		if a.Year > 0 {
			detailParts = append(detailParts, fmt.Sprintf("%d", a.Year))
		}
		if a.SongCount > 0 {
			detailParts = append(detailParts, fmt.Sprintf("%d tracks", a.SongCount))
		}
		detail := truncateStr(strings.Join(detailParts, " · "), iw)

		var borderColor color.Color = colorUnfocused
		nameStyle := lipgloss.NewStyle().Foreground(colorText)

		if selected {
			borderColor = colorFocused
			nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
		}

		content := nameStyle.Render(name) + "\n" +
			SubtextStyle.Render(artist) + "\n" +
			lipgloss.NewStyle().Foreground(colorDimText).Render(detail)

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
			indicator = lipgloss.NewStyle().Foreground(colorPlaying).Render("✓")
			if selected {
				borderColor = colorFocused
				nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
			} else {
				borderColor = colorUnfocused
				nameStyle = lipgloss.NewStyle().Foreground(colorText)
			}
			artistStyle = SubtextStyle
		} else {
			indicator = lipgloss.NewStyle().Foreground(colorDimText).Render("✗")
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
			indicator = lipgloss.NewStyle().Foreground(colorPlaying).Render("✓")
			if selected {
				borderColor = colorFocused
				nameStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
			} else {
				borderColor = colorUnfocused
				nameStyle = lipgloss.NewStyle().Foreground(colorText)
			}
			artistStyle = SubtextStyle
		} else {
			indicator = lipgloss.NewStyle().Foreground(colorDimText).Render("✗")
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
