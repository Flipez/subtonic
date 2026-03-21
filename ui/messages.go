package ui

import (
	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/sonos"
)

type TickMsg struct{}

type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastError
)

type ShowToastMsg struct {
	Text  string
	Level ToastLevel
}

type ToastExpiredMsg struct {
	ID int
}

type ArtistsLoadedMsg struct {
	Artists []api.Artist
	Err     error
}

type ArtistLoadedMsg struct {
	Artist *api.Artist
	Err    error
}

type AlbumLoadedMsg struct {
	Album *api.Album
	Err   error
}

type AlbumListLoadedMsg struct {
	Albums []api.Album
	Err    error
}

type PodcastsLoadedMsg struct {
	Podcasts []api.Podcast
	Err      error
}

type PodcastLoadedMsg struct {
	Podcast *api.Podcast
	Err     error
}

type SearchResultMsg struct {
	Result *api.SearchResult
	Err    error
}

type RandomSongsLoadedMsg struct {
	Songs []api.Song
	Err   error
}

type StarredLoadedMsg struct {
	Result *api.SearchResult
	Err    error
}

type GenresLoadedMsg struct {
	Genres []api.Genre
	Err    error
}

type SongsByGenreLoadedMsg struct {
	Songs []api.Song
	Err   error
}

type SimilarSongsLoadedMsg struct {
	Songs []api.Song
	Err   error
}

type TopSongsLoadedMsg struct {
	Songs []api.Song
	Err   error
}

type StarToggledMsg struct {
	ID      string
	Starred bool
	Err     error
}

type AllSongsLoadedMsg struct {
	Songs []api.Song
	Err   error
}

type DiscoverDataLoadedMsg struct {
	Recent   []api.Album
	Newest   []api.Album
	Frequent []api.Album
	Err      error
}

type DiscoverTrack struct {
	Title     string
	Artist    string
	Available bool
	Song      api.Song
}

type PlaylistsLoadedMsg struct {
	Playlists []api.Playlist
	Err       error
}

type PlaylistLoadedMsg struct {
	Playlist *api.Playlist
	Err      error
}

type PlaylistCreatedMsg struct {
	Playlist *api.Playlist
	Err      error
}

type PlaylistUpdatedMsg struct {
	Err error
}

type PlaylistDeletedMsg struct {
	ID  string
	Err error
}

type DiscoverRelease struct {
	Title     string
	Artist    string
	Date      string
	Available bool
	Album     api.Album
}

type LBTrendingLoadedMsg struct {
	Tracks []DiscoverTrack
	Err    error
}

type LBFreshReleasesLoadedMsg struct {
	Releases []DiscoverRelease
	Err      error
}

type LBPopularLoadedMsg struct {
	Tracks     []DiscoverTrack
	ArtistName string
	Err        error
}

type LBPlaylistLoadedMsg struct {
	Name   string
	Tracks []DiscoverTrack
	Kind   string
	Err    error
}

type LBRecommendedLoadedMsg struct {
	Tracks []DiscoverTrack
	Err    error
}

type SonosDiscoveredMsg struct {
	Devices []sonos.Device
	Err     error
}

// sonosSongEndedMsg is sent by the Sonos transport-state poll when the
// speaker reports STOPPED. Unlike player.SongEndedMsg (which comes from the
// local beep callback and is always authoritative), this message may arrive
// after the user has already switched back to local mode, so it is only acted
// on when IsSonosMode() is still true at handle time.
type sonosSongEndedMsg struct{}
