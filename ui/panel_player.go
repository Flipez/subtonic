package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/player"
)

type PlayerBar struct {
	song      *api.Song
	pos       time.Duration
	total     time.Duration
	paused    bool
	volume    int
	width     int // card-level width (for PlayerBarStyle.Width)
	audio     player.AudioInfo
	queueIdx  int
	queueLen  int
	shuffle   bool
	repeat    player.RepeatMode
	isSonos    bool
	sonosCount int

	gradient  []color.Color // cached Blend1D result; recomputed only on width change
	gradientW int
}

func NewPlayerBar(w int) PlayerBar {
	return PlayerBar{width: w, volume: 80}
}

// SetWidth sets the card-level width (includes padding, not border).
func (p *PlayerBar) SetWidth(w int) {
	p.width = w
}

func (p *PlayerBar) Update(song *api.Song, pos, total time.Duration, paused bool, vol int, audio player.AudioInfo, queueIdx, queueLen int, shuffle bool, repeat player.RepeatMode, isSonos bool, sonosCount int) {
	p.song = song
	p.pos = pos
	p.total = total
	p.paused = paused
	p.volume = vol
	p.audio = audio
	p.queueIdx = queueIdx
	p.queueLen = queueLen
	p.shuffle = shuffle
	p.repeat = repeat
	p.isSonos = isSonos
	p.sonosCount = sonosCount
}

func (p *PlayerBar) View() string {
	contentW := p.width - 4 // content area inside card (frame: 2 border + 2 padding)
	if contentW < 1 {
		contentW = 1
	}

	icon := IconPlay
	if p.paused || p.song == nil {
		icon = IconPause
	}

	// prefix: icon + queue position
	queueStr := ""
	if p.queueLen > 0 {
		queueStr = PlayerMetaStyle.Render(fmt.Sprintf("%d/%d", p.queueIdx+1, p.queueLen)) + "  "
	}
	prefix := PlayerIconStyle.Render(icon) + "  " + queueStr

	activeIndicator := lipgloss.NewStyle().Foreground(colorPlaying).Bold(true)

	// Fixed-width shuffle/repeat — always 1 cell so the layout never shifts.
	shuffleStr := PlayerMetaStyle.Render(" ")
	if p.shuffle {
		shuffleStr = activeIndicator.Render(IconShuffle)
	}
	repeatStr := PlayerMetaStyle.Render(" ")
	switch p.repeat {
	case player.RepeatAll:
		repeatStr = activeIndicator.Render(IconRepeatAll)
	case player.RepeatOne:
		repeatStr = activeIndicator.Render(IconRepeatOne)
	}

	volStr := renderVolume(p.volume)
	var rightParts []string
	if p.isSonos {
		rightParts = append(rightParts, activeIndicator.Render(fmt.Sprintf("%s %d Speakers", IconCast, p.sonosCount)))
	}
	rightParts = append(rightParts, shuffleStr, repeatStr, volStr)
	rightSide := strings.Join(rightParts, "  ")

	prefixW := lipgloss.Width(prefix)
	rightW := lipgloss.Width(rightSide)
	barRegionW := contentW - prefixW - rightW - 2 // 2 for gap before rightSide
	if barRegionW < 10 {
		barRegionW = 10
	}

	// Recompute gradient only when bar width changes.
	if p.gradientW != barRegionW {
		p.gradient = lipgloss.Blend1D(barRegionW, colorFocused, colorHighlight)
		p.gradientW = barRegionW
	}

	var line string
	if p.song != nil {
		pct := 0.0
		if p.total > 0 {
			pct = float64(p.pos) / float64(p.total)
		}
		timeStr := fmt.Sprintf("%s / %s", formatDuration(p.pos), formatDuration(p.total))
		barRegion := renderTrackProgressBar(p.song.Title, p.song.Artist, timeStr, pct, barRegionW, p.gradient)
		line = prefix + barRegion + "  " + rightSide
	} else {
		noTrack := SubtextStyle.Render("No track playing")
		gap := contentW - lipgloss.Width(prefix) - lipgloss.Width(noTrack) - rightW - 2
		if gap < 1 {
			gap = 1
		}
		line = prefix + noTrack + strings.Repeat(" ", gap) + "  " + rightSide
	}

	// Dynamic border color
	borderColor := colorUnfocused
	if p.song != nil {
		if p.paused {
			borderColor = colorFocused
		} else {
			borderColor = colorPlaying
		}
	}

	return PlayerBarStyle.Width(p.width).BorderForeground(borderColor).Render(line)
}

// renderTrackProgressBar renders a bar of `width` chars with track text left-aligned
// and timeStr right-aligned, both overlaid on the gradient fill.
func renderTrackProgressBar(title, artist, timeStr string, pct float64, width int, gradient []color.Color) string {
	timeRunes := []rune(timeStr)
	timeLen := len(timeRunes)
	rightStart := width - timeLen - 1

	// Track text fits left of the time with a 1-char gap.
	maxTrackW := rightStart - 1
	if maxTrackW < 0 {
		maxTrackW = 0
	}
	full := title
	if artist != "" {
		full += " · " + artist
	}
	text := truncateStr(" "+full, maxTrackW)
	textRunes := []rune(text)
	textLen := len(textRunes)

	fillEnd := int(pct * float64(width))
	if fillEnd > width {
		fillEnd = width
	}

	emptyTextStyle := lipgloss.NewStyle().Background(colorUnfocused).Foreground(colorSubtext)
	emptyBarStyle := lipgloss.NewStyle().Background(colorUnfocused)

	var b strings.Builder
	for i := 0; i < width; i++ {
		var ch string
		isText := false
		if i >= rightStart && (i-rightStart) < timeLen {
			ch = string(timeRunes[i-rightStart])
			isText = true
		} else if i < textLen {
			ch = string(textRunes[i])
			isText = true
		} else {
			ch = " "
		}

		if i < fillEnd {
			bg := gradient[i]
			if isText {
				b.WriteString(lipgloss.NewStyle().Background(bg).Foreground(colorUnfocused).Render(ch))
			} else {
				b.WriteString(lipgloss.NewStyle().Background(bg).Render(ch))
			}
		} else {
			if isText {
				b.WriteString(emptyTextStyle.Render(ch))
			} else {
				b.WriteString(emptyBarStyle.Render(ch))
			}
		}
	}
	return b.String()
}

// renderVolume produces a mini volume bar like "vol ████░ 80%"
func renderVolume(vol int) string {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	totalBlocks := 5
	filled := int(math.Round(float64(vol) / 100.0 * float64(totalBlocks)))
	empty := totalBlocks - filled

	filledStyle := lipgloss.NewStyle().Foreground(colorFocused)
	emptyStyle := lipgloss.NewStyle().Foreground(colorUnfocused)

	bar := filledStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty))
	return PlayerMetaStyle.Render(IconVolume+" ") + bar + PlayerMetaStyle.Render(fmt.Sprintf(" %3d%%", vol))
}

// padLine joins left and right text, filling the gap with spaces to reach width.
func padLine(left, right string, width int) string {
	if right == "" {
		return left
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatSize(bytes int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(mb))
	default:
		return fmt.Sprintf("%dKB", bytes/1024)
	}
}
