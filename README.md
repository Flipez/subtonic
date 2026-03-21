# subtonic

Terminal UI for Subsonic-compatible music servers.

> Requires a [Nerd Font](https://www.nerdfonts.com/) in your terminal for icons.

![Discover tab](docs/discover.png)

## Requirements

- A running Subsonic-compatible server (Navidrome, Airsonic, etc.)
- Go 1.21+
- A Nerd Font in your terminal emulator

## Install

```
go install github.com/Flipez/subtonic@latest
```

Or build from source:

```
git clone https://github.com/Flipez/subsonic-tui
cd subsonic-tui
go build -o subtonic .
```

## Configuration

On first run a config file is created at `~/.config/subtonic/config.toml`:

```toml
[server]
url      = "https://my-server.com"
username = "user"
password = "pass"

[player]
volume = 80
```

## Usage

```
subtonic
```

![Albums view](docs/albums.png)

### Navigation

| Key | Action |
|-----|--------|
| `1`–`6` | Switch tab (Home / Discover / Browse / Playlists / Podcasts / Search) |
| `enter` | Open / play |
| `esc` | Go back |
| `/` | Filter current list |
| `R` | Refresh |
| `?` | Show all keybindings |
| `q` | Quit |

### Playback

| Key | Action |
|-----|--------|
| `space` | Play / pause |
| `>` `.` | Next track |
| `<` `,` | Previous track |
| `=` | Volume +1% |
| `+` | Volume +5% |
| `-` | Volume -1% |
| `_` | Volume -5% |
| `s` | Toggle shuffle |
| `r` | Cycle repeat (off / all / one) |
| `Q` | Open queue |

Press `?` to see all keybindings in the app.

![Help popup](docs/help.png)

### Sonos

Press `o` to discover and connect to a Sonos speaker or group. Playback routes directly from the server to the speaker — the TUI controls what plays. Press `o` again to switch back to local audio. The current song and position carry over in both directions.

![Player bar with Sonos group](docs/sonos.png)

## Tabs

**Home** — Recently added albums displayed as a browsable grid. Navigate with arrow keys or `hjkl`.

**Discover** — Quick actions (random, starred, by genre, similar artists, top songs) plus ListenBrainz sections if a username is configured.

**Browse** — Library navigation: Artists, Albums (sorted newest-first), By Genre (sorted A–Z), and Starred items.

**Playlists** — Your playlists. Create, delete, and manage tracks.

**Podcasts** — Podcast subscriptions and episodes.

**Search** — Server-side search across artists, albums, and tracks.

![Search](docs/search.png)

## Player bar

The player bar is a single line showing:

- Play/pause state and queue position (e.g. `3/12`)
- A gradient progress bar with the track name and artist overlaid on the left, and the current position/duration on the right (e.g. `2:15 / 4:05`)
- Shuffle 󰒝 and repeat 󰑖/󰑗 indicators (always visible, fixed width)
- Volume level

## ListenBrainz

Optional. Add to config to enable trending tracks, fresh releases, and personalised recommendations:

```toml
[listenbrainz]
username = "your-lb-username"
token    = "your-lb-token"   # only needed for recommendations
```
