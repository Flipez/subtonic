# 🍸 subtonic

Terminal UI for Subsonic-compatible music servers.

![Home tab](docs/discover.png)

## Requirements

- A running Subsonic-compatible server (Navidrome, Airsonic, etc.)
- Go 1.21+
- A [Nerd Font](https://www.nerdfonts.com/) in your terminal emulator

## Install

```
go install github.com/Flipez/subtonic@latest
```

Or build from source:

```
git clone https://github.com/Flipez/subtonic
cd subtonic
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

Press `o` to discover and connect to a Sonos speaker or group. Playback routes directly from the server to the speaker — the TUI controls what plays. Press `o` again to switch back to local audio.

![Player bar with Sonos group](docs/sonos.png)

## ListenBrainz

Optional. Add to config to enable trending tracks, fresh releases, and personalised recommendations:

```toml
[listenbrainz]
username = "your-lb-username"
token    = "your-lb-token"   # only needed for recommendations
```

## Screenshots

![Albums view](docs/albums.png)

![Discover tab](docs/listenbrainz.png)

![Search](docs/search.png)