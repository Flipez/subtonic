package ui

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/listenbrainz"
	"github.com/Flipez/subtonic/sonos"
)

func discoverSonosCmd() tea.Cmd {
	return func() tea.Msg {
		devices, err := sonos.Discover(3 * time.Second)
		return SonosDiscoveredMsg{Devices: devices, Err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

func loadArtistsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		artists, err := client.GetArtists()
		return ArtistsLoadedMsg{Artists: artists, Err: err}
	}
}

func loadArtistCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		artist, err := client.GetArtist(id)
		return ArtistLoadedMsg{Artist: artist, Err: err}
	}
}

func loadAlbumListCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		albums, err := client.GetAlbumList2("alphabeticalByName", 500, 0)
		return AlbumListLoadedMsg{Albums: albums, Err: err}
	}
}

func loadAlbumCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		album, err := client.GetAlbum(id)
		return AlbumLoadedMsg{Album: album, Err: err}
	}
}


func loadPodcastsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		podcasts, err := client.GetPodcasts()
		return PodcastsLoadedMsg{Podcasts: podcasts, Err: err}
	}
}

func loadPodcastCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		podcast, err := client.GetPodcast(id)
		return PodcastLoadedMsg{Podcast: podcast, Err: err}
	}
}

func searchCmd(client *api.Client, query string) tea.Cmd {
	return func() tea.Msg {
		result, err := client.Search(query)
		return SearchResultMsg{Result: result, Err: err}
	}
}

func loadRandomSongsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		songs, err := client.GetRandomSongs(50)
		return RandomSongsLoadedMsg{Songs: songs, Err: err}
	}
}

func loadStarredCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		result, err := client.GetStarred2()
		return StarredLoadedMsg{Result: result, Err: err}
	}
}

func loadGenresCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		genres, err := client.GetGenres()
		return GenresLoadedMsg{Genres: genres, Err: err}
	}
}

func loadSongsByGenreCmd(client *api.Client, genre string) tea.Cmd {
	return func() tea.Msg {
		const pageSize = 500
		var all []api.Song
		for offset := 0; ; offset += pageSize {
			page, err := client.GetSongsByGenre(genre, pageSize, offset)
			if err != nil {
				return SongsByGenreLoadedMsg{Err: err}
			}
			all = append(all, page...)
			if len(page) < pageSize {
				break
			}
		}
		return SongsByGenreLoadedMsg{Songs: all}
	}
}

func loadSimilarSongsCmd(client *api.Client, songID, artistID string) tea.Cmd {
	return func() tea.Msg {
		// Try getSimilarSongs (song ID) first, fall back to getSimilarSongs2 (artist ID)
		songs, err := client.GetSimilarSongs(songID, 50)
		if (err != nil || len(songs) == 0) && artistID != "" {
			songs2, err2 := client.GetSimilarSongs2(artistID, 50)
			if err2 == nil && len(songs2) > 0 {
				return SimilarSongsLoadedMsg{Songs: songs2, Err: nil}
			}
		}
		return SimilarSongsLoadedMsg{Songs: songs, Err: err}
	}
}

func loadTopSongsCmd(client *api.Client, artist string) tea.Cmd {
	return func() tea.Msg {
		songs, err := client.GetTopSongs(artist, 50)
		return TopSongsLoadedMsg{Songs: songs, Err: err}
	}
}

func starToggleCmd(client *api.Client, id string, currentlyStarred bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if currentlyStarred {
			err = client.Unstar(id)
		} else {
			err = client.Star(id)
		}
		return StarToggledMsg{ID: id, Starred: !currentlyStarred, Err: err}
	}
}


func loadDiscoverDataCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		recent, err1 := client.GetAlbumList2("recent", 5, 0)
		newest, err2 := client.GetAlbumList2("newest", 5, 0)
		frequent, err3 := client.GetAlbumList2("frequent", 5, 0)
		return DiscoverDataLoadedMsg{Recent: recent, Newest: newest, Frequent: frequent, Err: errors.Join(err1, err2, err3)}
	}
}


var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	var lines []string
	line := ""
	for _, word := range words {
		if line == "" {
			line = word
		} else if len(line)+1+len(word) <= width {
			line += " " + word
		} else {
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func loadInfoCmd(client *api.Client, kind, id, title string) tea.Cmd {
	return func() tea.Msg {
		switch kind {
		case "artist":
			info, err := client.GetArtistInfo2(id)
			if err != nil {
				return InfoLoadedMsg{Err: err}
			}
			if info == nil || info.Biography == "" {
				return InfoLoadedMsg{Title: title, Content: "No biography available."}
			}
			return InfoLoadedMsg{Title: title, Content: stripHTML(info.Biography)}
		case "album":
			info, err := client.GetAlbumInfo2(id)
			if err != nil {
				return InfoLoadedMsg{Err: err}
			}
			if info == nil || info.Notes == "" {
				return InfoLoadedMsg{Title: title, Content: "No information available."}
			}
			return InfoLoadedMsg{Title: title, Content: stripHTML(info.Notes)}
		}
		return InfoLoadedMsg{Err: fmt.Errorf("unknown kind: %s", kind)}
	}
}

func stringMatchesFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func matchTrack(subsonic *api.Client, artist, title string) DiscoverTrack {
	dt := DiscoverTrack{
		Title:  title,
		Artist: artist,
	}
	// Try combined query first, then title-only as fallback for search
	queries := []string{artist + " " + title, title}
	for _, query := range queries {
		songs, err := subsonic.SearchSongs(query, 10)
		if err != nil || len(songs) == 0 {
			continue
		}
		// Exact match on both title and artist
		for _, s := range songs {
			if strings.EqualFold(s.Title, title) && strings.EqualFold(s.Artist, artist) {
				dt.Available = true
				dt.Song = s
				return dt
			}
		}
		// Substring containment on both title and artist
		for _, s := range songs {
			if stringMatchesFold(s.Title, title) && stringMatchesFold(s.Artist, artist) {
				dt.Available = true
				dt.Song = s
				return dt
			}
		}
	}
	return dt
}

func matchRelease(subsonic *api.Client, artist, albumTitle, date string) DiscoverRelease {
	dr := DiscoverRelease{
		Title:  albumTitle,
		Artist: artist,
		Date:   date,
	}
	result, err := subsonic.Search(artist + " " + albumTitle)
	if err != nil || result == nil {
		return dr
	}
	for _, a := range result.Albums {
		if strings.EqualFold(a.Name, albumTitle) && strings.EqualFold(a.Artist, artist) {
			dr.Available = true
			dr.Album = a
			return dr
		}
	}
	// Fallback: substring match
	for _, a := range result.Albums {
		if stringMatchesFold(a.Name, albumTitle) && stringMatchesFold(a.Artist, artist) {
			dr.Available = true
			dr.Album = a
			return dr
		}
	}
	return dr
}

func loadLBTrendingCmd(lb *listenbrainz.Client, subsonic *api.Client) tea.Cmd {
	return func() tea.Msg {
		recordings, err := lb.GetTrendingRecordings(20)
		if err != nil {
			return LBTrendingLoadedMsg{Err: err}
		}
		var result []DiscoverTrack
		for _, r := range recordings {
			result = append(result, matchTrack(subsonic, r.ArtistName, r.TrackName))
		}
		return LBTrendingLoadedMsg{Tracks: result}
	}
}

func loadLBFreshReleasesCmd(lb *listenbrainz.Client, subsonic *api.Client) tea.Cmd {
	return func() tea.Msg {
		releases, err := lb.GetFreshReleases()
		if err != nil {
			return LBFreshReleasesLoadedMsg{Err: err}
		}
		var result []DiscoverRelease
		for _, r := range releases {
			result = append(result, matchRelease(subsonic, r.ArtistName, r.Title, r.Date))
		}
		return LBFreshReleasesLoadedMsg{Releases: result}
	}
}

func loadLBPopularCmd(lb *listenbrainz.Client, subsonic *api.Client, artist, track string) tea.Cmd {
	return func() tea.Msg {
		recordings, err := lb.GetPopularByArtist(artist, track)
		if err != nil {
			return LBPopularLoadedMsg{Err: err, ArtistName: artist}
		}
		var result []DiscoverTrack
		for _, r := range recordings {
			result = append(result, matchTrack(subsonic, r.ArtistName, r.TrackName))
		}
		return LBPopularLoadedMsg{Tracks: result, ArtistName: artist}
	}
}

func loadLBCreatedForPlaylistsCmd(lb *listenbrainz.Client, subsonic *api.Client, username string) tea.Cmd {
	return func() tea.Msg {
		playlists, err := lb.GetPlaylistsCreatedFor(username)
		if err != nil {
			return LBCreatedForPlaylistsLoadedMsg{Err: err}
		}
		// Fetch full playlist data in parallel (createdfor endpoint returns empty tracks)
		fetched := make([]*listenbrainz.Playlist, len(playlists))
		var wg sync.WaitGroup
		for i := range playlists {
			p := &playlists[i]
			if len(p.Tracks) == 0 && p.MBID != "" {
				wg.Add(1)
				go func(idx int, mbid string) {
					defer wg.Done()
					full, err := lb.GetPlaylist(mbid)
					if err == nil && full != nil {
						fetched[idx] = full
					}
				}(i, p.MBID)
			} else {
				fetched[i] = p
			}
		}
		wg.Wait()

		var result []LBCreatedForPlaylist
		for _, p := range fetched {
			if p == nil {
				continue
			}
			var tracks []DiscoverTrack
			for _, t := range p.Tracks {
				if t.TrackName != "" && t.ArtistName != "" {
					tracks = append(tracks, matchTrack(subsonic, t.ArtistName, t.TrackName))
				}
			}
			if len(tracks) > 0 {
				result = append(result, LBCreatedForPlaylist{Name: p.Title, Tracks: tracks})
			}
		}
		return LBCreatedForPlaylistsLoadedMsg{Playlists: result}
	}
}

func loadPlaylistsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.GetPlaylists()
		return PlaylistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func loadPlaylistCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		playlist, err := client.GetPlaylist(id)
		return PlaylistLoadedMsg{Playlist: playlist, Err: err}
	}
}

func createPlaylistCmd(client *api.Client, name string) tea.Cmd {
	return func() tea.Msg {
		playlist, err := client.CreatePlaylist(name)
		return PlaylistCreatedMsg{Playlist: playlist, Err: err}
	}
}

func addToPlaylistCmd(client *api.Client, id string, songIDs []string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdatePlaylist(id, songIDs)
		return PlaylistUpdatedMsg{Err: err}
	}
}

func deletePlaylistCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeletePlaylist(id)
		return PlaylistDeletedMsg{ID: id, Err: err}
	}
}

func removeFromPlaylistCmd(client *api.Client, id string, indices []int) tea.Cmd {
	return func() tea.Msg {
		err := client.RemoveFromPlaylist(id, indices)
		return PlaylistUpdatedMsg{Err: err}
	}
}

func loadPlaylistShuffleCmd(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		pl, err := client.GetPlaylist(id)
		if err != nil || pl == nil {
			return AllSongsLoadedMsg{}
		}
		return AllSongsLoadedMsg{Songs: pl.Songs}
	}
}

func loadLBRecommendedCmd(lb *listenbrainz.Client, subsonic *api.Client, username string) tea.Cmd {
	return func() tea.Msg {
		recordings, err := lb.GetRecommendations(username, 20)
		if err != nil {
			return LBRecommendedLoadedMsg{Err: err}
		}
		var result []DiscoverTrack
		for _, r := range recordings {
			result = append(result, matchTrack(subsonic, r.ArtistName, r.TrackName))
		}
		return LBRecommendedLoadedMsg{Tracks: result}
	}
}

func loadArtistAllSongsCmd(client *api.Client, artistID string) tea.Cmd {
	return func() tea.Msg {
		artist, err := client.GetArtist(artistID)
		if err != nil {
			return AllSongsLoadedMsg{Err: err}
		}
		if artist == nil {
			return AllSongsLoadedMsg{}
		}
		var songs []api.Song
		for _, album := range artist.Albums {
			a, err := client.GetAlbum(album.ID)
			if err != nil {
				continue
			}
			if a != nil {
				songs = append(songs, a.Songs...)
			}
		}
		return AllSongsLoadedMsg{Songs: songs}
	}
}

func loadAlbumsAllSongsCmd(client *api.Client, albums []api.Album) tea.Cmd {
	return func() tea.Msg {
		var songs []api.Song
		for _, album := range albums {
			a, err := client.GetAlbum(album.ID)
			if err != nil {
				continue
			}
			if a != nil {
				songs = append(songs, a.Songs...)
			}
		}
		return AllSongsLoadedMsg{Songs: songs}
	}
}
