# subtonic — Feature Backlog

## Playback & Queue

- [ ] Seek control — expose internal seeking via keybinds (e.g. `f`/`b` for ±10s)
- [ ] Clear queue — single action to empty the queue
- [ ] Save queue as playlist — capture current session
- [ ] Gapless playback — seamless transitions between tracks
- [ ] Crossfade — optional transition effect between tracks
- [ ] Sleep timer — stop playback after N minutes

## Browsing & Navigation

- [ ] Sort toggles — sort albums/songs by year, play count, date added (not just alphabetical)
- [ ] Configurable table columns — show/hide artist, year, bitrate, duration, etc.
- [ ] Pagination controls — currently hardcoded to 500 items; add "load more"
- [ ] Search history — retain previous search queries

## Playlist Management

- [ ] Rename playlist
- [ ] Reorder songs in playlist — `J`/`K` like queue view
- [ ] Bulk operations — multi-select to delete/move/star multiple items
- [ ] Duplicate detection/removal
- [ ] Playlist description editing

## Discovery & Recommendations

- [ ] Play history — "Last N songs played" view (Subsonic scrobbles this)
- [ ] Radio/station mode — endless similar songs from a seed track via `getSimilarSongs`
- [ ] Lyrics display — Subsonic `getLyrics` endpoint, show in popup like info view
- [ ] Album art — render cover art (metadata available, not displayed)
- [ ] ListenBrainz scrobble history — view recent listens from LB profile

## Sonos

- [ ] Individual speaker volume control within groups
- [ ] View/edit Sonos queue

## Configuration & Persistence

- [ ] Multiple server profiles — switch between servers without editing config
- [ ] Keybinding customization
- [ ] Theme/color customization
- [ ] Transcode/bitrate selection — expose Subsonic transcoding settings
- [ ] Resume queue on startup — persist queue + position across sessions

## Integrations

- [ ] Last.fm scrobbling
- [ ] MPRIS / desktop notifications (now playing)
- [ ] Cover art from MusicBrainz/CoverArtArchive via MBID
