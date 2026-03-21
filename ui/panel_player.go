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
	isSonos   bool
	sonosName string
}

func NewPlayerBar(w int) PlayerBar {
	return PlayerBar{width: w, volume: 80}
}

// SetWidth sets the card-level width (includes padding, not border).
func (p *PlayerBar) SetWidth(w int) {
	p.width = w
}

func (p *PlayerBar) Update(song *api.Song, pos, total time.Duration, paused bool, vol int, audio player.AudioInfo, queueIdx, queueLen int, shuffle bool, repeat player.RepeatMode, isSonos bool, sonosName string) {
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
	p.sonosName = sonosName
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

	// right side: time + vol + indicators
	timeStr := PlayerTimeStyle.Render(fmt.Sprintf("%s / %s", formatDuration(p.pos), formatDuration(p.total)))
	volStr := renderVolume(p.volume)

	activeIndicator := lipgloss.NewStyle().Foreground(colorPlaying).Bold(true)
	var indicators []string
	if p.isSonos {
		name := p.sonosName
		if name == "" {
			name = "Sonos"
		}
		indicators = append(indicators, activeIndicator.Render(fmt.Sprintf("%s %s", IconCast, name)))
	}
	if p.shuffle {
		indicators = append(indicators, activeIndicator.Render(IconShuffle))
	}
	switch p.repeat {
	case player.RepeatAll:
		indicators = append(indicators, activeIndicator.Render(IconRepeatAll))
	case player.RepeatOne:
		indicators = append(indicators, activeIndicator.Render(IconRepeatOne))
	}
	rightParts := append(indicators, timeStr, volStr)
	rightSide := strings.Join(rightParts, "  ")

	prefixW := lipgloss.Width(prefix)
	rightW := lipgloss.Width(rightSide)
	barRegionW := contentW - prefixW - rightW - 2 // 2 for gap before rightSide
	if barRegionW < 10 {
		barRegionW = 10
	}

	var line string
	if p.song != nil {
		pct := 0.0
		if p.total > 0 {
			pct = float64(p.pos) / float64(p.total)
		}
		barRegion := renderTrackProgressBar(p.song.Title, p.song.Artist, pct, barRegionW)
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

// renderTrackProgressBar renders a bar of `width` chars with the track text overlaid on top.
// The filled portion has a gradient from colorFocused (left) to colorHighlight (right).
func renderTrackProgressBar(title, artist string, pct float64, width int) string {
	full := title
	if artist != "" {
		full += " · " + artist
	}
	text := truncateStr(" "+full, width)
	runes := []rune(text)
	textLen := len(runes)
	fillEnd := int(pct * float64(width))
	if fillEnd > width {
		fillEnd = width
	}

	emptyTextStyle := lipgloss.NewStyle().Background(colorUnfocused).Foreground(colorSubtext)
	emptyBarStyle := lipgloss.NewStyle().Background(colorUnfocused)

	var b strings.Builder
	for i := 0; i < width; i++ {
		var ch string
		if i < textLen {
			ch = string(runes[i])
		} else {
			ch = " "
		}
		if i < fillEnd {
			// gradient: colorFocused → colorHighlight across full bar width
			t := float64(i) / float64(max(width-1, 1))
			bg := lerpColor(colorFocused, colorHighlight, t)
			if i < textLen {
				b.WriteString(lipgloss.NewStyle().Background(bg).Foreground(colorUnfocused).Render(ch))
			} else {
				b.WriteString(lipgloss.NewStyle().Background(bg).Render(ch))
			}
		} else {
			if i < textLen {
				b.WriteString(emptyTextStyle.Render(ch))
			} else {
				b.WriteString(emptyBarStyle.Render(ch))
			}
		}
	}
	return b.String()
}

// lerpColor linearly interpolates between two colors, t in [0,1].
func lerpColor(c1, c2 color.Color, t float64) color.Color {
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()
	r := uint8((float64(r1) + t*(float64(r2)-float64(r1))) / 257.0)
	g := uint8((float64(g1) + t*(float64(g2)-float64(g1))) / 257.0)
	b := uint8((float64(b1) + t*(float64(b2)-float64(b1))) / 257.0)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
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
	return PlayerMetaStyle.Render(IconVolume+" ") + bar + PlayerMetaStyle.Render(fmt.Sprintf(" %d%%", vol))
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
