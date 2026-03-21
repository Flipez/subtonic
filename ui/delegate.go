package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
)

// Item types

type artistItem struct{ api.Artist }

func (i artistItem) FilterValue() string { return i.Artist.Name }

type albumItem struct{ api.Album }

func (i albumItem) FilterValue() string { return i.Album.Name + " " + i.Album.Artist }

type playlistItem struct{ api.Playlist }

func (i playlistItem) FilterValue() string { return i.Playlist.Name }

type songItem struct{ api.Song }

func (i songItem) FilterValue() string { return i.Song.Title + " " + i.Song.Artist }

// segment represents a styled piece of text in a row.
type segment struct {
	text string
	dim  bool
}

// CompactDelegate renders single-line list items with fixed-width columns.

type CompactDelegate struct {
	selected    lipgloss.Style
	normal      lipgloss.Style
	dimNormal   lipgloss.Style
	dimSelected lipgloss.Style
}

func NewCompactDelegate() CompactDelegate {
	return CompactDelegate{
		selected: lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true),
		normal: lipgloss.NewStyle().
			Foreground(colorText),
		dimNormal: lipgloss.NewStyle().
			Foreground(colorSubtext),
		dimSelected: lipgloss.NewStyle().
			Foreground(colorHighlight),
	}
}

func (d CompactDelegate) Height() int                             { return 1 }
func (d CompactDelegate) Spacing() int                            { return 0 }
func (d CompactDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Column width constants. Every fixed column uses a constant so that
// all rows in a list compute the same fixedW and variable columns align.
const (
	starW    = 2  // "★ " or "  "
	trackW   = 5  // "   02" or " 2-01"
	yearW    = 4  // "2020"
	durW     = 5  // " 4:32"
	bitrateW = 5  // " 320k" or "1411k"
	suffixW  = 4  // "FLAC"
	albumsW  = 10 // " 12 albums"
	tracksW  = 10 // " 12 tracks"
	gap      = 2  // "  " between columns
)

func (d CompactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	isSelected := index == m.Index()
	width := m.Width()
	if width < 10 {
		width = 10
	}

	var segments []segment
	switch it := item.(type) {
	case artistItem:
		// ★ Name                       12 albums
		star := starPrefix(it.Artist.Starred)
		albumsStr := fmt.Sprintf("%d albums", it.Artist.AlbumCount)
		fixedW := starW + gap + albumsW
		nameW := width - fixedW
		if nameW < 8 {
			nameW = 8
		}
		segments = []segment{
			{star, false},
			{col(it.Artist.Name, nameW), false},
			{"  ", true},
			{rCol(albumsStr, albumsW), true},
		}

	case albumItem:
		// ★ Name          Artist    Genre      2020   12 tracks
		star := starPrefix(it.Album.Starred)
		yearStr := "    "
		if it.Album.Year > 0 {
			yearStr = fmt.Sprintf("%4d", it.Album.Year)
		}
		tracksStr := fmt.Sprintf("%d tracks", it.Album.SongCount)

		fixedW := starW + gap + gap + gap + yearW + gap + tracksW
		remaining := width - fixedW
		nameW := remaining * 40 / 100
		artistW := remaining * 30 / 100
		genreW := remaining - nameW - artistW
		if nameW < 8 {
			nameW = 8
		}
		if artistW < 6 {
			artistW = 6
		}
		if genreW < 4 {
			genreW = 4
		}
		segments = []segment{
			{star, false},
			{col(it.Album.Name, nameW), false},
			{"  ", false},
			{col(it.Album.Artist, artistW), false},
			{"  ", true},
			{col(it.Album.Genre, genreW), true},
			{"  ", false},
			{yearStr, false},
			{"  ", true},
			{rCol(tracksStr, tracksW), true},
		}

	case songItem:
		// 02  Title           Artist       4:32  320k FLAC
		trackStr := formatTrackNumber(it.Song.Track, it.Song.DiscNumber)
		durStr := fmt.Sprintf("%d:%02d", it.Song.Duration/60, it.Song.Duration%60)
		suffixStr := strings.ToUpper(it.Song.Suffix)
		bitrateStr := ""
		if it.Song.BitRate > 0 {
			bitrateStr = fmt.Sprintf("%dk", it.Song.BitRate)
		}

		fixedW := trackW + gap + gap + gap + durW + 1 + bitrateW + 1 + suffixW
		remaining := width - fixedW
		titleW := remaining * 55 / 100
		artistW := remaining - titleW
		if titleW < 8 {
			titleW = 8
		}
		if artistW < 6 {
			artistW = 6
		}
		segments = []segment{
			{rCol(trackStr, trackW), false},
			{"  ", false},
			{col(it.Song.Title, titleW), false},
			{"  ", false},
			{col(it.Song.Artist, artistW), false},
			{"  ", true},
			{rCol(durStr, durW), true},
			{" ", true},
			{rCol(bitrateStr, bitrateW), true},
			{" ", true},
			{col(suffixStr, suffixW), true},
		}

	case playlistItem:
		dur := formatDurationCompact(it.Playlist.Duration)
		right := fmt.Sprintf("%d tracks  %s", it.Playlist.SongCount, dur)
		nameW := width - len(right) - gap
		if nameW < 8 {
			nameW = 8
		}
		segments = []segment{
			{col(it.Playlist.Name, nameW), false},
			{"  ", true},
			{right, true},
		}

	default:
		fmt.Fprint(w, d.normal.Render(item.FilterValue()))
		return
	}

	d.renderSegments(w, segments, isSelected, width)
}

// renderSegments concatenates styled segments and writes the result.
func (d CompactDelegate) renderSegments(w io.Writer, segs []segment, selected bool, width int) {
	var b strings.Builder
	for _, s := range segs {
		var style lipgloss.Style
		if selected {
			if s.dim {
				style = d.dimSelected
			} else {
				style = d.selected
			}
		} else {
			if s.dim {
				style = d.dimNormal
			} else {
				style = d.normal
			}
		}
		b.WriteString(style.Render(s.text))
	}
	line := b.String()
	if lipgloss.Width(line) > width {
		line = truncate(line, width)
	}
	fmt.Fprint(w, line)
}

func starPrefix(starred string) string {
	if starred != "" {
		return "★ "
	}
	return "  "
}

func formatTrackNumber(track, disc int) string {
	if disc > 1 {
		return fmt.Sprintf("%d-%02d", disc, track)
	}
	return fmt.Sprintf("%02d", track)
}

// col pads or truncates text to exactly w visible characters (left-aligned).
func col(text string, w int) string {
	if w <= 0 {
		return ""
	}
	visible := lipgloss.Width(text)
	if visible > w {
		return truncate(text, w)
	}
	return text + strings.Repeat(" ", w-visible)
}

// rCol pads or truncates text to exactly w visible characters (right-aligned).
func rCol(text string, w int) string {
	if w <= 0 {
		return ""
	}
	visible := lipgloss.Width(text)
	if visible > w {
		return truncate(text, w)
	}
	return strings.Repeat(" ", w-visible) + text
}

// truncate cuts text to fit in maxW visible characters, adding ellipsis.
func truncate(text string, maxW int) string {
	if maxW <= 1 {
		return text[:maxW]
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxW-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func formatDurationCompact(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
