package player

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/sonos"
)

const sampleRate = beep.SampleRate(48000)

type SongEndedMsg struct{}

type Player struct {
	mu      sync.Mutex
	queue   Queue
	ctrl    *beep.Ctrl
	vol     *effects.Volume
	current *api.Song
	stream  beep.StreamSeekCloser
	format  beep.Format
	program *tea.Program

	volumePct int
	paused    bool

	// Sonos output mode
	sonosClient   *sonos.Client
	sonosMode     bool
	sonosSong     *api.Song
	sonosPaused   bool
	sonosStopPoll chan struct{} // closed to stop the background progress poller
	// position cache written by background goroutine, read by Progress()
	sonosPosCache time.Duration
	sonosDurCache time.Duration
}

func New() (*Player, error) {
	if err := speaker.Init(sampleRate, sampleRate.N(time.Millisecond*100)); err != nil {
		return nil, fmt.Errorf("speaker init: %w", err)
	}
	return &Player{volumePct: 80}, nil
}

func (p *Player) SetProgram(prog *tea.Program) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.program = prog
}

func (p *Player) Queue() *Queue {
	return &p.queue
}

// Play starts streaming and playing the given song from the beginning.
func (p *Player) Play(song api.Song, streamURL string) error {
	return p.playAt(song, streamURL, 0)
}

// PlayFrom starts playback at approximately seekPos.
//
// For MP3: issues an HTTP Range request so the server starts the byte stream
// near the right position. MP3 decoders scan for frame-sync bytes and tolerate
// arbitrary start offsets, so this is safe and fast.
//
// For FLAC/OGG and other container formats: Range requests break the file
// structure (the decoder needs its magic header at byte 0). Instead the full
// stream is downloaded from the start and the decoder is seeked forward after
// decoding begins. This works if the decoder buffers the file; at worst it
// falls back silently to playing from the beginning.
func (p *Player) PlayFrom(song api.Song, streamURL string, seekPos time.Duration) error {
	if seekPos <= 0 {
		return p.playAt(song, streamURL, 0)
	}

	suffix := strings.ToLower(song.Suffix)
	isMp3 := suffix == "mp3" || suffix == ""

	if isMp3 && song.BitRate > 0 {
		// BitRate is kbps: bytes/s = kbps * 1000 / 8 = kbps * 125
		byteOffset := int64(seekPos.Seconds()) * int64(song.BitRate) * 125
		return p.playAt(song, streamURL, byteOffset)
	}

	// FLAC, OGG, etc: play from start then seek the decoder.
	if err := p.playAt(song, streamURL, 0); err != nil {
		return err
	}
	p.SeekTo(seekPos) // best-effort; silently stays at 0 if decoder can't seek
	return nil
}

// FreshSonosPosition queries Sonos directly for the current position,
// bypassing the 2-second background-poll cache. Falls back to the cache on error.
func (p *Player) FreshSonosPosition() time.Duration {
	p.mu.Lock()
	if !p.sonosMode || p.sonosClient == nil {
		v := p.sonosPosCache
		p.mu.Unlock()
		return v
	}
	client := p.sonosClient
	p.mu.Unlock()

	pos, _, err := client.GetPositionInfo()
	if err != nil {
		p.mu.Lock()
		v := p.sonosPosCache
		p.mu.Unlock()
		return v
	}
	return pos
}

func (p *Player) playAt(song api.Song, streamURL string, byteOffset int64) error {
	// Sonos mode: tell the speaker to fetch and play the stream directly.
	p.mu.Lock()
	sonosMode := p.sonosMode
	sonosClient := p.sonosClient
	p.mu.Unlock()

	if sonosMode {
		if err := sonosClient.SetURI(streamURL, song.Title, song.Artist, song.Album, song.Suffix); err != nil {
			return fmt.Errorf("sonos set uri: %w", err)
		}
		if err := sonosClient.Play(); err != nil {
			return fmt.Errorf("sonos play: %w", err)
		}
		p.mu.Lock()
		s := song
		p.sonosSong = &s
		p.sonosPaused = false
		p.sonosPosCache = 0
		p.sonosDurCache = 0
		p.mu.Unlock()
		return nil
	}

	// Phase 1: network + decode — no locks held.
	// Connection timeout, but no read deadline (stream runs for the full song).
	streamClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("stream request: %w", err)
	}
	if byteOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", byteOffset))
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream fetch: %w", err)
	}
	// 200 = full file (server ignored Range), 206 = partial from byteOffset
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return fmt.Errorf("stream HTTP %d", resp.StatusCode)
	}

	// Prefer Content-Type over file suffix: Subsonic often transcodes on the
	// fly and the suffix reflects the original file, not the wire format.
	codec := codecFromContentType(resp.Header.Get("Content-Type"))
	if codec == "" {
		codec = strings.ToLower(song.Suffix)
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch codec {
	case "mp3":
		streamer, format, err = mp3.Decode(resp.Body)
	case "ogg", "oga", "opus":
		streamer, format, err = vorbis.Decode(resp.Body)
	case "flac":
		streamer, format, err = flac.Decode(resp.Body)
	default:
		streamer, format, err = mp3.Decode(resp.Body)
	}
	if err != nil {
		resp.Body.Close()
		return fmt.Errorf("decode %s (codec=%s): %w", song.Suffix, codec, err)
	}

	var resampled beep.Streamer = streamer
	if format.SampleRate != sampleRate {
		resampled = beep.Resample(3, format.SampleRate, sampleRate, streamer)
	}

	ctrl := &beep.Ctrl{Streamer: resampled, Paused: false}
	vol := &effects.Volume{Streamer: ctrl, Base: 2}

	// Phase 2: swap state under p.mu (brief, no speaker calls).
	p.mu.Lock()
	oldStream := p.stream
	p.stream = streamer
	p.format = format
	p.ctrl = ctrl
	p.vol = vol
	p.vol.Volume = p.dbVolume()
	p.vol.Silent = p.volumePct == 0
	p.current = &song
	p.paused = false
	p.mu.Unlock()

	// Phase 3: close old stream after releasing p.mu.
	if oldStream != nil {
		oldStream.Close()
	}

	// Phase 4: hand off to speaker.
	// speaker.Clear() and speaker.Play() each acquire the speaker mutex
	// internally — do NOT wrap them in speaker.Lock/Unlock (self-deadlock).
	seq := beep.Seq(vol, beep.Callback(p.onSongEnd))
	speaker.Clear()
	speaker.Play(seq)
	return nil
}

// onSongEnd is called from inside beep's Callback while the speaker goroutine
// holds speaker.mu. It must not call speaker.Lock() or any func that does.
// p.mu is safe here: TogglePause releases p.mu before calling speaker.Lock(),
// so there is no p.mu ↔ speaker.mu lock-order inversion.
func (p *Player) onSongEnd() {
	p.mu.Lock()
	prog := p.program
	p.mu.Unlock()
	if prog != nil {
		prog.Send(SongEndedMsg{})
	}
}

// TogglePause pauses or resumes playback.
// p.mu is released BEFORE calling speaker.Lock() to prevent the lock-order
// inversion: onSongEnd holds speaker.mu and then acquires p.mu.
func (p *Player) TogglePause() {
	p.mu.Lock()
	if p.sonosMode {
		p.sonosPaused = !p.sonosPaused
		paused := p.sonosPaused
		client := p.sonosClient
		p.mu.Unlock()
		if paused {
			client.Pause() //nolint:errcheck
		} else {
			client.Play() //nolint:errcheck
		}
		return
	}
	if p.ctrl == nil {
		p.mu.Unlock()
		return
	}
	p.paused = !p.paused
	paused := p.paused
	ctrl := p.ctrl
	p.mu.Unlock()

	speaker.Lock()
	ctrl.Paused = paused
	speaker.Unlock()
}

// Stop halts playback and closes the current stream.
func (p *Player) Stop() {
	p.mu.Lock()
	if p.sonosMode {
		client := p.sonosClient
		p.sonosSong = nil
		p.mu.Unlock()
		client.Stop() //nolint:errcheck
		return
	}
	old := p.stream
	p.stream = nil
	p.ctrl = nil
	p.current = nil
	p.mu.Unlock()

	// speaker.Clear() locks internally.
	speaker.Clear()

	if old != nil {
		old.Close()
	}
}

// SetVolume sets playback volume 0–100.
// p.mu is released BEFORE calling speaker.Lock() (same reason as TogglePause).
func (p *Player) SetVolume(pct int) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	p.mu.Lock()
	if p.sonosMode {
		client := p.sonosClient
		p.volumePct = pct
		p.mu.Unlock()
		client.SetVolume(pct) //nolint:errcheck
		return
	}
	p.volumePct = pct
	vol := p.vol
	db := p.dbVolume()
	silent := pct == 0
	p.mu.Unlock()

	if vol != nil {
		speaker.Lock()
		vol.Volume = db
		vol.Silent = silent
		speaker.Unlock()
	}
}

func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volumePct
}

func (p *Player) dbVolume() float64 {
	if p.volumePct <= 0 {
		return -10 // well below audible; Silent flag handles true mute
	}
	// Quadratic amplitude law: amplitude = (pct/100)^2
	// Converts to beep's Base-2 volume field via log2.
	// Results: 50% → -12 dB, 10% → -40 dB, 5% → -52 dB.
	return 2.0 * math.Log2(float64(p.volumePct)/100.0)
}

// codecFromContentType maps an HTTP Content-Type header to a codec name.
func codecFromContentType(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return "mp3"
	case strings.Contains(ct, "ogg"), strings.Contains(ct, "vorbis"):
		return "ogg"
	case strings.Contains(ct, "flac"):
		return "flac"
	case strings.Contains(ct, "opus"):
		return "opus"
	}
	return ""
}

func (p *Player) Progress() (pos, total time.Duration) {
	p.mu.Lock()
	if p.sonosMode {
		// Cache is written by the background polling goroutine; no SOAP call here.
		pos, total = p.sonosPosCache, p.sonosDurCache
		p.mu.Unlock()
		return pos, total
	}
	defer p.mu.Unlock()
	if p.stream == nil || p.format.SampleRate == 0 {
		return 0, 0
	}
	posN := p.stream.Position()
	total = p.format.SampleRate.D(p.stream.Len())
	pos = p.format.SampleRate.D(posN)
	// Streaming decoders may not know the total length; fall back to song metadata
	if total == 0 && p.current != nil && p.current.Duration > 0 {
		total = time.Duration(p.current.Duration) * time.Second
	}
	return pos, total
}

func (p *Player) CurrentSong() *api.Song {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sonosMode {
		return p.sonosSong
	}
	return p.current
}

func (p *Player) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sonosMode {
		return p.sonosPaused
	}
	return p.paused
}

// AudioInfo holds technical details about the current audio stream.
type AudioInfo struct {
	SampleRate  int  // e.g. 44100, 48000, 96000
	BitDepth    int  // bytes per sample * 8, e.g. 16, 24
	Channels    int  // 1 = mono, 2 = stereo
	Resampling  bool // true if original rate differs from output
}

func (p *Player) AudioInfo() AudioInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sonosMode {
		return AudioInfo{} // no local decode info in Sonos mode
	}
	if p.format.SampleRate == 0 {
		return AudioInfo{}
	}
	return AudioInfo{
		SampleRate: int(p.format.SampleRate),
		BitDepth:   p.format.Precision * 8,
		Channels:   p.format.NumChannels,
		Resampling: p.format.SampleRate != sampleRate,
	}
}

// EnableSonos stops local playback and switches to Sonos output mode.
// It starts a background goroutine that polls GetPositionInfo every 2s so
// that Progress() can return cached values without blocking Update().
func (p *Player) EnableSonos(client *sonos.Client) {
	// Stop any existing poll goroutine.
	p.mu.Lock()
	if p.sonosStopPoll != nil {
		close(p.sonosStopPoll)
	}
	old := p.stream
	p.stream = nil
	p.ctrl = nil
	p.current = nil
	p.sonosClient = client
	p.sonosMode = true
	p.sonosSong = nil
	p.sonosPaused = false
	p.sonosPosCache = 0
	p.sonosDurCache = 0
	stop := make(chan struct{})
	p.sonosStopPoll = stop
	p.mu.Unlock()

	speaker.Clear()
	if old != nil {
		old.Close()
	}
	go p.sonosPollLoop(client, stop)
}

// sonosPollLoop polls Sonos for playback position every 2s and updates the
// cache. Runs until stop is closed.
func (p *Player) sonosPollLoop(client *sonos.Client, stop <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			pos, dur, err := client.GetPositionInfo()
			if err != nil {
				continue
			}
			p.mu.Lock()
			song := p.sonosSong
			if dur == 0 && song != nil && song.Duration > 0 {
				dur = time.Duration(song.Duration) * time.Second
			}
			p.sonosPosCache = pos
			p.sonosDurCache = dur
			p.mu.Unlock()
		}
	}
}

// DisableSonos stops Sonos playback and returns to local audio mode.
func (p *Player) DisableSonos() {
	p.mu.Lock()
	if p.sonosStopPoll != nil {
		close(p.sonosStopPoll)
		p.sonosStopPoll = nil
	}
	client := p.sonosClient
	p.sonosMode = false
	p.sonosClient = nil
	p.sonosSong = nil
	p.sonosPaused = false
	p.mu.Unlock()

	if client != nil {
		client.Stop() //nolint:errcheck
	}
}

// SeekTo seeks the current stream to pos. In Sonos mode this calls the
// AVTransport Seek action; in local mode it seeks the beep stream (best-effort
// — HTTP-backed streams may not support it).
func (p *Player) SeekTo(pos time.Duration) {
	if pos <= 0 {
		return
	}
	p.mu.Lock()
	if p.sonosMode {
		client := p.sonosClient
		p.sonosPosCache = pos
		p.mu.Unlock()
		client.Seek(pos) //nolint:errcheck
		return
	}
	stream := p.stream
	format := p.format
	p.mu.Unlock()

	if stream == nil || format.SampleRate == 0 {
		return
	}
	samplePos := int(pos.Seconds() * float64(format.SampleRate))
	speaker.Lock()
	stream.Seek(samplePos) //nolint:errcheck
	speaker.Unlock()
}

// IsSonosMode reports whether Sonos output mode is active.
func (p *Player) IsSonosMode() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sonosMode
}

// SonosTransportState returns the current Sonos transport state string
// ("PLAYING", "PAUSED_PLAYBACK", "STOPPED") for queue advancement polling.
func (p *Player) SonosTransportState() (string, error) {
	p.mu.Lock()
	client := p.sonosClient
	p.mu.Unlock()
	if client == nil {
		return "", nil
	}
	return client.GetTransportInfo()
}

// SonosDeviceName returns the friendly name of the connected Sonos speaker.
func (p *Player) SonosDeviceName() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sonosClient == nil {
		return ""
	}
	return p.sonosClient.DeviceName()
}
