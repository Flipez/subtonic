package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/player"
)

type PlayerBar struct {
	progress   progress.Model
	song       *api.Song
	pos        time.Duration
	total      time.Duration
	paused     bool
	volume     int
	width      int // card-level width (for PlayerBarStyle.Width)
	audio      player.AudioInfo
	queueIdx   int
	queueLen   int
	shuffle    bool
	repeat     player.RepeatMode
	isSonos    bool
	sonosName  string
}

func NewPlayerBar(w int) PlayerBar {
	prog := progress.New(
		progress.WithColors(colorFocused, colorHighlight),
		progress.WithoutPercentage(),
	)
	return PlayerBar{progress: prog, width: w, volume: 80}
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

	icon := "▶"
	if p.paused || p.song == nil {
		icon = "⏸"
	}

	var line1, line2 string

	sep := SubtextStyle.Render(" · ")

	if p.song != nil {
		// Line 1: icon + title · album · artist (left) | queue · shuffle/repeat · specs · star (right)
		leftParts := []string{NowPlayingStyle.Render(p.song.Title)}
		if p.song.Album != "" {
			leftParts = append(leftParts, PlayerAlbumStyle.Render(p.song.Album))
		}
		if p.song.Artist != "" {
			leftParts = append(leftParts, SubtextStyle.Render(p.song.Artist))
		}
		left1 := PlayerIconStyle.Render(icon) + "  " + strings.Join(leftParts, sep)

		activeIndicator := lipgloss.NewStyle().Foreground(colorPlaying).Bold(true)
		var rightParts []string
		if p.isSonos {
			name := p.sonosName
			if name == "" {
				name = "Sonos"
			}
			rightParts = append(rightParts, activeIndicator.Render(fmt.Sprintf("[Sonos: %s]", name)))
		}
		if p.queueLen > 0 {
			rightParts = append(rightParts, PlayerMetaStyle.Render(fmt.Sprintf("%d/%d", p.queueIdx+1, p.queueLen)))
		}
		if p.shuffle {
			rightParts = append(rightParts, activeIndicator.Render("[shuffle]"))
		}
		switch p.repeat {
		case player.RepeatAll:
			rightParts = append(rightParts, activeIndicator.Render("[repeat]"))
		case player.RepeatOne:
			rightParts = append(rightParts, activeIndicator.Render("[repeat 1]"))
		}
		if specs := p.renderAudioSpecs(); specs != "" {
			rightParts = append(rightParts, specs)
		}
		if p.song.Starred != "" {
			rightParts = append(rightParts, "★")
		}
		var right1 string
		if len(rightParts) > 0 {
			right1 = strings.Join(rightParts, sep)
		}
		line1 = padLine(left1, right1, contentW)
	} else {
		line1 = PlayerIconStyle.Render(icon) + "  " + SubtextStyle.Render("No track playing")
	}

	// Line 2: progress bar + time + volume
	pct := 0.0
	if p.total > 0 {
		pct = float64(p.pos) / float64(p.total)
	}

	timeStr := PlayerTimeStyle.Render(fmt.Sprintf("%s / %s", formatDuration(p.pos), formatDuration(p.total)))
	volStr := renderVolume(p.volume)
	rightSide := timeStr + "  " + volStr

	barW := contentW - lipgloss.Width(rightSide) - 2
	if barW < 10 {
		barW = 10
	}
	p.progress.SetWidth(barW)
	bar := p.progress.ViewAs(pct)
	line2 = bar + "  " + rightSide

	content := lipgloss.JoinVertical(lipgloss.Left, line1, "", line2)

	// Dynamic border color
	borderColor := colorUnfocused
	if p.song != nil {
		if p.paused {
			borderColor = colorFocused
		} else {
			borderColor = colorPlaying
		}
	}

	return PlayerBarStyle.Width(p.width).BorderForeground(borderColor).Render(content)
}

// renderAudioSpecs builds a string like "FLAC 96.0kHz/24bit stereo 14.2MB ⟳"
func (p *PlayerBar) renderAudioSpecs() string {
	if p.song == nil || p.audio.SampleRate == 0 {
		return ""
	}

	var parts []string

	// Codec
	if p.song.Suffix != "" {
		parts = append(parts, strings.ToUpper(p.song.Suffix))
	}

	// Sample rate + bit depth
	sr := float64(p.audio.SampleRate) / 1000.0
	var srStr string
	if sr == float64(int(sr)) {
		srStr = fmt.Sprintf("%gkHz", sr)
	} else {
		srStr = fmt.Sprintf("%.1fkHz", sr)
	}
	if p.audio.BitDepth > 0 {
		srStr += fmt.Sprintf("/%dbit", p.audio.BitDepth)
	}
	parts = append(parts, srStr)

	// Channels
	switch p.audio.Channels {
	case 1:
		parts = append(parts, "mono")
	case 2:
		parts = append(parts, "stereo")
	default:
		if p.audio.Channels > 0 {
			parts = append(parts, fmt.Sprintf("%dch", p.audio.Channels))
		}
	}

	// File size
	if p.song.Size > 0 {
		parts = append(parts, formatSize(p.song.Size))
	}

	dimMeta := lipgloss.NewStyle().Foreground(colorDimText)
	return dimMeta.Render(strings.Join(parts, "  "))
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
	return PlayerMetaStyle.Render("vol ") + bar + PlayerMetaStyle.Render(fmt.Sprintf(" %d%%", vol))
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
