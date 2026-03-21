# subtonic — project context for Claude

## Module

`github.com/Flipez/subtonic` (repo is `subsonic-tui`, module name differs)

## Stack

- **TUI**: `charm.land/bubbletea/v2` (BubbleTea v2), `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`
- **Audio**: `gopxl/beep/v2` — MP3, FLAC, OGG/Vorbis via `ebitengine/oto` (requires CGO + libasound2-dev on Linux)
- **Config**: `BurntSushi/toml`, stored at `~/.config/subtonic/config.toml`
- **Fuzzy filter**: `sahilm/fuzzy`

## Package layout

| Package | Purpose |
|---------|---------|
| `api/` | Subsonic API client (token auth, all endpoints) |
| `config/` | TOML config load/save |
| `player/` | Audio playback (beep), Sonos output mode, queue |
| `sonos/` | UPnP/SSDP discovery, SOAP AVTransport/RenderingControl |
| `listenbrainz/` | ListenBrainz API (trending, recommendations, playlists) |
| `ui/` | All TUI code — BubbleTea model, views, panels |

## Architecture

- Single BubbleTea `Model` in `ui/model.go` (~2600 lines) — all state lives here
- Async data loading via `tea.Cmd` goroutines returning typed `Msg` values
- `player.Player` is mutex-safe; audio runs in a beep speaker goroutine
- 500ms tick drives progress bar updates and Sonos state polling
- Overlays (toast, help popup) rendered via `lipgloss.NewCompositor` + `lipgloss.NewCanvas`

## Key design decisions

- `relayout()` must be called whenever the number of visible input lines changes (search input, filter input, new-playlist input) — it recalculates the table height
- `Canvas.Compose(layer)` ignores layer X/Y — always use `NewCompositor(layers...)` then `canvas.Compose(comp)` for positioned overlays
- Volume uses quadratic amplitude law: `beepVolume = 2 * log2(pct/100)` — 5% ≈ −52 dB, 50% ≈ −12 dB
- Sonos position polling runs in a background goroutine (2s interval); `Progress()` reads from cache only — no SOAP calls in the UI tick
- For Sonos→local switch: use `FreshSonosPosition()` (direct SOAP call) then `PlayFrom()` which uses HTTP Range for MP3 and decoder seek for FLAC/OGG

## Sonos implementation notes

- Discovery: SSDP M-SEARCH → fetch device XML (namespace-strip before parsing) → ZoneGroupTopology for groups
- Groups: send `x-rincon:COORDINATOR_UUID` to each non-coordinator member before setting stream URI, to prevent firmware dropping group sync
- DIDL-Lite metadata required in `SetAVTransportURI` — without it Sonos returns UPnP error 714
- `soapCall` returns error on HTTP 500 (SOAP fault); fault message is parsed and surfaced

## UI layout (line heights)

```
header card:   3 lines  (border×2 + nav line)
breadcrumb:    1 line
content card:  dynamic  (reduced by 1 per active input widget)
player card:   5 lines  (border×2 + line1 + blank + line2)
help bar:      1 line
```

`relayout()` subtracts active input lines (TabSearch always −1, local filter −1, new-playlist −1) before calling `table.SetHeight`.

## Conventions

- No new dependencies without good reason — stdlib preferred
- `//nolint:errcheck` on fire-and-forget Sonos calls (Pause, Stop, group sync)
- Toast popups: top-right corner, Y=1. Help popup: centered. Both via Compositor.
- Help bar shows only view-local keys; global keys live in the `?` popup
- `renderToastPopup()` not `renderToast()` — the old line-based renderToast is gone
