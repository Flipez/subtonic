package ui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/sonos"
)

func (m *Model) buildTable() {
	var cols []table.Column
	var rows []table.Row

	switch m.viewType {
	case ViewDiscover:
		cols, rows = m.buildDiscoverTable()
	case ViewGenres:
		cols, rows = m.buildGenresTable()
	case ViewArtists:
		cols, rows = m.buildArtistsTable()
	case ViewAlbums:
		cols, rows = m.buildAlbumsTable()
	case ViewPlaylists, ViewPlaylistPicker:
		cols, rows = m.buildPlaylistsTable()
	case ViewSongs:
		cols, rows = m.buildSongsTable()
	case ViewPodcasts:
		cols, rows = m.buildPodcastsTable()
	case ViewEpisodes:
		cols, rows = m.buildEpisodesTable()
	case ViewSearchResults:
		cols, rows = m.buildSearchResultsTable()
	case ViewQueue:
		cols, rows = m.buildQueueTable()
	case ViewSonosPicker:
		cols, rows = m.buildSonosPickerTable()
	case ViewBrowse:
		cols, rows = m.buildBrowseTable()
	}

	// Clear rows first so UpdateViewport (triggered by SetColumns) doesn't
	// render old rows against the new column layout.
	m.table.SetRows(nil)
	m.table.SetColumns(cols)
	m.table.SetRows(rows)
}

func (m *Model) buildDiscoverTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	options, _ := m.viewData.([]DiscoverOption)
	numCols := 2
	padding := 2 * numCols
	descW := 40
	modeW := contentW - descW - padding
	if modeW < 10 {
		modeW = 10
	}
	cols := []table.Column{
		{Title: "Mode", Width: modeW},
		{Title: "Description", Width: descW},
	}
	rows := make([]table.Row, len(options))
	for i, option := range options {
		rows[i] = table.Row{option.Label, option.Description}
	}
	return cols, rows
}

func (m *Model) buildGenresTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	genres, _ := m.viewData.([]api.Genre)
	slices.SortFunc(genres, func(a, b api.Genre) int { return strings.Compare(a.Name, b.Name) })
	numCols := 3
	padding := 2 * numCols
	songsW := 8
	albumsW := 8
	fixedW := songsW + albumsW
	nameW := contentW - fixedW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Genre", Width: nameW},
		{Title: "Songs", Width: songsW},
		{Title: "Albums", Width: albumsW},
	}
	rows := make([]table.Row, len(genres))
	for i, genre := range genres {
		rows[i] = table.Row{genre.Name, fmt.Sprintf("%d", genre.SongCount), fmt.Sprintf("%d", genre.AlbumCount)}
	}
	return cols, rows
}

func (m *Model) buildArtistsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	artists, _ := m.viewData.([]api.Artist)
	numCols := 2
	padding := 2 * numCols
	albumsColW := 8
	nameW := contentW - albumsColW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Albums", Width: albumsColW},
	}
	rows := make([]table.Row, len(artists))
	for i, artist := range artists {
		rows[i] = table.Row{artist.Name, fmt.Sprintf("%d", artist.AlbumCount)}
	}
	return cols, rows
}

func (m *Model) buildAlbumsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	albums, _ := m.viewData.([]api.Album)
	slices.SortFunc(albums, func(a, b api.Album) int {
		if a.Year == b.Year {
			return 0
		}
		if a.Year == 0 {
			return 1 // zero year goes last
		}
		if b.Year == 0 {
			return -1
		}
		return b.Year - a.Year // descending: newest first
	})
	numCols := 4
	padding := 2 * numCols
	yearW := 4
	tracksW := 8
	fixedW := yearW + tracksW
	remaining := contentW - fixedW - padding
	if remaining < 20 {
		remaining = 20
	}
	nameW := remaining * 50 / 100
	artistW := remaining - nameW
	cols := []table.Column{
		{Title: "Year", Width: yearW},
		{Title: "Name", Width: nameW},
		{Title: "Artist", Width: artistW},
		{Title: "Tracks", Width: tracksW},
	}
	rows := make([]table.Row, len(albums))
	for i, album := range albums {
		yearStr := ""
		if album.Year > 0 {
			yearStr = fmt.Sprintf("%d", album.Year)
		}
		rows[i] = table.Row{yearStr, album.Name, album.Artist, fmt.Sprintf("%d", album.SongCount)}
	}
	return cols, rows
}

func (m *Model) buildPlaylistsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	playlists, _ := m.viewData.([]api.Playlist)
	numCols := 2
	padding := 2 * numCols
	tracksW := 8
	nameW := contentW - tracksW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Tracks", Width: tracksW},
	}
	rows := make([]table.Row, len(playlists))
	for i, playlist := range playlists {
		rows[i] = table.Row{playlist.Name, fmt.Sprintf("%d", playlist.SongCount)}
	}
	return cols, rows
}

func (m *Model) buildSongsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	songs, _ := m.viewData.([]api.Song)
	showAlbum := m.activeTab == TabPlaylists
	dimStyle := lipgloss.NewStyle().Foreground(colorDimText)
	heartW := 1

	heart := func(song api.Song) string {
		if song.Starred != "" {
			return IconHeart
		}
		return ""
	}

	if showAlbum {
		numCols := 6
		padding := 2 * numCols
		trackW := 3
		timeW := 6
		fixedW := trackW + timeW + heartW
		remaining := contentW - fixedW - padding
		if remaining < 30 {
			remaining = 30
		}
		titleW := remaining * 35 / 100
		artistW := remaining * 30 / 100
		albumW := remaining - titleW - artistW
		cols := []table.Column{
			{Title: "#", Width: trackW},
			{Title: "Title", Width: titleW},
			{Title: "Artist", Width: artistW},
			{Title: "Album", Width: albumW},
			{Title: "Time", Width: timeW},
			{Title: IconHeart, Width: heartW},
		}
		rows := make([]table.Row, len(songs))
		for i, song := range songs {
			if song.ID == "" {
				rows[i] = table.Row{
					dimStyle.Render(IconClose),
					dimStyle.Render(song.Title),
					dimStyle.Render(song.Artist),
					dimStyle.Render(song.Album),
					"", "",
				}
				continue
			}
			durStr := fmt.Sprintf("%d:%02d", song.Duration/60, song.Duration%60)
			rows[i] = table.Row{
				formatTrackNumber(song.Track, song.DiscNumber),
				song.Title,
				song.Artist,
				song.Album,
				durStr,
				heart(song),
			}
		}
		return cols, rows
	}

	numCols := 5
	padding := 2 * numCols
	trackW := 5
	timeW := 6
	fixedW := trackW + timeW + heartW
	remaining := contentW - fixedW - padding
	if remaining < 20 {
		remaining = 20
	}
	titleW := remaining * 55 / 100
	artistW := remaining - titleW
	cols := []table.Column{
		{Title: "#", Width: trackW},
		{Title: "Title", Width: titleW},
		{Title: "Artist", Width: artistW},
		{Title: "Time", Width: timeW},
		{Title: IconHeart, Width: heartW},
	}
	rows := make([]table.Row, len(songs))
	for i, song := range songs {
		trackStr := formatTrackNumber(song.Track, song.DiscNumber)
		durStr := fmt.Sprintf("%d:%02d", song.Duration/60, song.Duration%60)
		if song.ID == "" {
			rows[i] = table.Row{
				dimStyle.Render(IconClose),
				dimStyle.Render(song.Title),
				dimStyle.Render(song.Artist),
				"", "",
			}
			continue
		}
		rows[i] = table.Row{
			trackStr,
			song.Title,
			song.Artist,
			durStr,
			heart(song),
		}
	}
	return cols, rows
}

func (m *Model) buildPodcastsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	podcasts, _ := m.viewData.([]api.Podcast)
	numCols := 2
	padding := 2 * numCols
	epsW := 10
	nameW := contentW - epsW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Podcast", Width: nameW},
		{Title: "Episodes", Width: epsW},
	}
	rows := make([]table.Row, len(podcasts))
	for i, podcast := range podcasts {
		available := 0
		for _, ep := range podcast.Episodes {
			if ep.Status == "completed" {
				available++
			}
		}
		rows[i] = table.Row{podcast.Title, fmt.Sprintf("%d/%d", available, len(podcast.Episodes))}
	}
	return cols, rows
}

func (m *Model) buildEpisodesTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	episodes, _ := m.viewData.([]api.PodcastEpisode)
	numCols := 3
	padding := 2 * numCols
	durW := 8
	statusW := 10
	fixedW := durW + statusW
	nameW := contentW - fixedW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Episode", Width: nameW},
		{Title: "Duration", Width: durW},
		{Title: "Status", Width: statusW},
	}
	rows := make([]table.Row, len(episodes))
	for i, episode := range episodes {
		durStr := ""
		if episode.Duration > 0 {
			durStr = formatDurationCompact(episode.Duration)
		}
		rows[i] = table.Row{episode.Title, durStr, episode.Status}
	}
	return cols, rows
}

func (m *Model) buildSearchResultsTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	items, _ := m.viewData.([]SearchResultItem)
	numCols := 3
	padding := 2 * numCols
	typeW := 8
	detailW := 20
	fixedW := typeW + detailW
	nameW := contentW - fixedW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Type", Width: typeW},
		{Title: "Name", Width: nameW},
		{Title: "Detail", Width: detailW},
	}
	rows := make([]table.Row, len(items))
	for i, item := range items {
		switch item.Type {
		case "artist":
			rows[i] = table.Row{"Artist", item.Artist.Name, fmt.Sprintf("%d albums", item.Artist.AlbumCount)}
		case "album":
			rows[i] = table.Row{"Album", item.Album.Name, item.Album.Artist}
		case "song":
			durStr := fmt.Sprintf("%d:%02d", item.Song.Duration/60, item.Song.Duration%60)
			rows[i] = table.Row{"Track", item.Song.Title, item.Song.Artist + "  " + durStr}
		}
	}
	return cols, rows
}

func (m *Model) buildQueueTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	songs := m.pl.Queue().Songs()
	queueIdx := m.pl.Queue().Index()
	numCols := 5
	padding := 2 * numCols
	markerW := 2
	trackW := 5
	timeW := 6
	fixedW := markerW + trackW + timeW
	remaining := contentW - fixedW - padding
	if remaining < 20 {
		remaining = 20
	}
	titleW := remaining * 55 / 100
	artistW := remaining - titleW
	cols := []table.Column{
		{Title: "", Width: markerW},
		{Title: "#", Width: trackW},
		{Title: "Title", Width: titleW},
		{Title: "Artist", Width: artistW},
		{Title: "Time", Width: timeW},
	}
	rows := make([]table.Row, len(songs))
	for i, song := range songs {
		marker := " "
		if i == queueIdx {
			marker = IconPlay
		}
		durStr := fmt.Sprintf("%d:%02d", song.Duration/60, song.Duration%60)
		rows[i] = table.Row{
			marker,
			fmt.Sprintf("%d", i+1),
			song.Title,
			song.Artist,
			durStr,
		}
	}
	return cols, rows
}

func (m *Model) buildSonosPickerTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	devices, _ := m.viewData.([]sonos.Device)
	nameW := contentW - 4
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Sonos Speaker", Width: nameW},
	}
	rows := make([]table.Row, len(devices))
	for i, device := range devices {
		rows[i] = table.Row{device.Name}
	}
	return cols, rows
}

func (m *Model) buildBrowseTable() ([]table.Column, []table.Row) {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	numCols := 2
	padding := 2 * numCols
	descW := 40
	nameW := contentW - descW - padding
	if nameW < 10 {
		nameW = 10
	}
	cols := []table.Column{
		{Title: "Browse", Width: nameW},
		{Title: "Description", Width: descW},
	}
	rows := make([]table.Row, len(browseOptions))
	for i, opt := range browseOptions {
		rows[i] = table.Row{opt.Label, opt.Description}
	}
	return cols, rows
}
