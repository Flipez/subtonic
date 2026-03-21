package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/config"
	"github.com/Flipez/subtonic/listenbrainz"
	"github.com/Flipez/subtonic/player"
	"github.com/Flipez/subtonic/sonos"
)

type Tab int

const (
	TabDiscover Tab = iota
	TabArtists
	TabAlbums
	TabPlaylists
	TabPodcasts
	TabSearch
)

type ViewType int

const (
	ViewArtists ViewType = iota
	ViewAlbums
	ViewPlaylists
	ViewSongs
	ViewPodcasts
	ViewEpisodes
	ViewSearchResults
	ViewDiscover
	ViewGenres
	ViewQueue
	ViewPlaylistPicker
	ViewSonosPicker
)

// SearchResultItem is a tagged union for mixed search results.
type SearchResultItem struct {
	Type   string // "artist", "album", "song"
	Artist api.Artist
	Album  api.Album
	Song   api.Song
}

// DiscoverOption represents a discovery mode on the Discover tab.
type DiscoverOption struct {
	Label       string
	Description string
	Action      string // "random", "starred", "genres", "similar", "top"
}

var discoverOptions = []DiscoverOption{
	{Label: "Random Songs", Description: "Shuffle across entire library", Action: "random"},
	{Label: "Starred", Description: "Favorited artists, albums, and songs", Action: "starred"},
	{Label: "By Genre", Description: "Pick a genre, get songs", Action: "genres"},
	{Label: "Similar", Description: "Based on currently playing song", Action: "similar"},
	{Label: "Top Songs", Description: "Top tracks of currently playing artist", Action: "top"},
}


type NavLevel struct {
	Label    string
	ViewType ViewType
	Data     any // []api.Artist, []api.Album, []api.Playlist, or []api.Song
	Cursor   int
}

type toast struct {
	id    int
	text  string
	level ToastLevel
}

type Model struct {
	width, height int
	ready         bool

	// Navigation
	activeTab Tab
	navStack  []NavLevel

	// Table (used for all views)
	table    table.Model
	viewType ViewType
	viewData any // []api.Artist, []api.Album, []api.Playlist, or []api.Song

	// Local fuzzy filter
	searching   bool
	searchInput textinput.Model
	searchData  any // saved unfiltered viewData during search

	// Server search (search tab)
	serverSearching bool
	serverInput     textinput.Model

	// Player
	bar PlayerBar

	// Loading
	loading bool
	spinner spinner.Model

	// Toasts
	toasts   []toast
	toastSeq int

	// Dependencies
	api *api.Client
	pl  *player.Player
	cfg config.Config

	// Cached data
	artists   []api.Artist
	albumList []api.Album
	podcasts  []api.Podcast

	// Discover landing page
	discoverRecent   []api.Album
	discoverNewest   []api.Album
	discoverFrequent []api.Album
	discoverSection  int // which section is focused
	discoverItem     int // which item within section

	// ListenBrainz integration
	lb              *listenbrainz.Client
	lbTrending      []DiscoverTrack
	lbFreshReleases []DiscoverRelease
	lbPopular       []DiscoverTrack
	lbPopularArtist string
	lbDailyJams     []DiscoverTrack
	lbDailyJamsName string
	lbWeekly        []DiscoverTrack
	lbWeeklyName    string
	lbRecommended   []DiscoverTrack
	lbCurrentArtist string
	// Playlists (Subsonic)
	playlists         []api.Playlist
	pendingSongIDs    []string
	pickerInput       textinput.Model
	creatingNew       bool
	deleteConfirmID   string
	currentPlaylistID string

	// Queue view state
	prevView     ViewType
	prevViewData any
	prevCursor   int

	// Sonos output mode
	sonosDevices []sonos.Device

	// Help popup
	showHelp bool
}

func New(client *api.Client, pl *player.Player, cfg config.Config, lb *listenbrainz.Client) Model {
	input := textinput.New()
	input.Placeholder = "Filter..."
	input.CharLimit = 100

	serverInput := textinput.New()
	serverInput.Placeholder = "Search artists, albums, tracks..."
	serverInput.CharLimit = 200

	pickerInput := textinput.New()
	pickerInput.Placeholder = "Playlist name..."
	pickerInput.CharLimit = 100

	sp := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(colorHighlight)),
	)

	t := table.New(
		table.WithFocused(true),
		table.WithStyles(SongTableStyles()),
	)
	km := table.DefaultKeyMap()
	km.PageDown = key.NewBinding(key.WithKeys("pgdown", "f", "ctrl+f"), key.WithHelp("pgdn", "page down"))
	t.KeyMap = km

	m := Model{
		api:         client,
		pl:          pl,
		cfg:         cfg,
		lb:          lb,
		activeTab:   TabDiscover,
		table:       t,
		searchInput: input,
		serverInput: serverInput,
		pickerInput: pickerInput,
		bar:         NewPlayerBar(0),
		spinner:     sp,
		viewType:    ViewDiscover,
		viewData:    discoverOptions,
	}

	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadArtistsCmd(m.api),
		loadAlbumListCmd(m.api),
		loadPodcastsCmd(m.api),
		loadPlaylistsCmd(m.api),
		loadDiscoverDataCmd(m.api),
		tickCmd(),
		loadLBTrendingCmd(m.lb, m.api),
		loadLBFreshReleasesCmd(m.lb, m.api),
	}
	if m.lb.HasUsername() {
		cmds = append(cmds,
			loadLBPlaylistCmd(m.lb, m.api, m.lb.Username(), "daily-jams"),
			loadLBPlaylistCmd(m.lb, m.api, m.lb.Username(), "weekly-exploration"),
		)
	}
	if m.lb.HasAuth() {
		cmds = append(cmds, loadLBRecommendedCmd(m.lb, m.api, m.lb.Username()))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		m.ready = true
		return m, nil

	case tea.KeyPressMsg:
		// Help popup: any key closes it; ? toggles.
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// When server search input is active, intercept keys
		if m.serverSearching {
			return m.updateServerSearch(msg)
		}

		// When local fuzzy filtering, intercept all keys
		if m.searching {
			return m.updateSearch(msg)
		}

		// When creating a new playlist (text input)
		if m.creatingNew {
			return m.updateCreatePlaylist(msg)
		}

		// When confirming playlist deletion
		if m.deleteConfirmID != "" {
			if key.Matches(msg, GlobalKeys.Delete) {
				id := m.deleteConfirmID
				m.deleteConfirmID = ""
				m.loading = true
				return m, tea.Batch(deletePlaylistCmd(m.api, id), m.spinner.Tick)
			}
			m.deleteConfirmID = ""
			return m, nil
		}

		// When playlist picker is open
		if m.viewType == ViewPlaylistPicker {
			return m.updatePlaylistPicker(msg)
		}

		// When Sonos device picker is open
		if m.viewType == ViewSonosPicker {
			return m.updateSonosPicker(msg)
		}

		// Global keys
		switch {
		case key.Matches(msg, GlobalKeys.Quit):
			if m.viewType == ViewQueue || m.viewType == ViewPlaylistPicker {
				return m.closeOverlay()
			}
			m.cfg.Player.Volume = m.pl.Volume()
			config.Save(m.cfg) //nolint:errcheck
			m.pl.Stop()
			return m, tea.Quit

		case key.Matches(msg, GlobalKeys.PlayPause):
			pl := m.pl
			return m, func() tea.Msg {
				pl.TogglePause()
				return TickMsg{}
			}

		case key.Matches(msg, GlobalKeys.Next):
			return m, m.nextTrack()

		case key.Matches(msg, GlobalKeys.Prev):
			return m, m.prevTrack()

		case key.Matches(msg, GlobalKeys.VolUp):
			pl := m.pl
			return m, func() tea.Msg {
				pl.SetVolume(pl.Volume() + 1)
				return TickMsg{}
			}

		case key.Matches(msg, GlobalKeys.VolDown):
			pl := m.pl
			return m, func() tea.Msg {
				pl.SetVolume(pl.Volume() - 1)
				return TickMsg{}
			}

		case key.Matches(msg, GlobalKeys.VolUpLarge):
			pl := m.pl
			return m, func() tea.Msg {
				pl.SetVolume(pl.Volume() + 5)
				return TickMsg{}
			}

		case key.Matches(msg, GlobalKeys.VolDownLarge):
			pl := m.pl
			return m, func() tea.Msg {
				pl.SetVolume(pl.Volume() - 5)
				return TickMsg{}
			}

		case key.Matches(msg, GlobalKeys.Search):
			if m.viewType == ViewQueue {
				return m, nil
			}
			if m.activeTab == TabSearch {
				m.serverSearching = true
				m.serverInput.Focus()
				return m, nil
			}
			m.searching = true
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.searchData = m.viewData
			m.relayout()
			return m, nil

		case key.Matches(msg, GlobalKeys.Back):
			if m.viewType == ViewQueue || m.viewType == ViewPlaylistPicker {
				return m.closeOverlay()
			}
			return m.popNav()

		case key.Matches(msg, GlobalKeys.Shuffle):
			m.pl.Queue().ToggleShuffle()
			label := "Shuffle off"
			if m.pl.Queue().Shuffle() {
				label = "Shuffle on"
			}
			return m, m.showToast(label, ToastInfo, 2*time.Second)

		case key.Matches(msg, GlobalKeys.Repeat):
			m.pl.Queue().CycleRepeat()
			var label string
			switch m.pl.Queue().Repeat() {
			case player.RepeatOff:
				label = "Repeat off"
			case player.RepeatAll:
				label = "Repeat all"
			case player.RepeatOne:
				label = "Repeat one"
			}
			return m, m.showToast(label, ToastInfo, 2*time.Second)

		case key.Matches(msg, GlobalKeys.Star):
			return m.handleStar()

		case key.Matches(msg, GlobalKeys.Queue):
			if m.viewType == ViewQueue {
				return m.closeOverlay()
			}
			return m.handleOpenQueue()

		case key.Matches(msg, GlobalKeys.Delete):
			if m.viewType == ViewQueue {
				return m.handleQueueRemove()
			}
			if m.viewType == ViewPlaylists && m.activeTab == TabPlaylists && len(m.navStack) == 0 {
				return m.handleDeletePlaylist()
			}
			if m.viewType == ViewSongs && m.currentPlaylistID != "" {
				return m.handleRemoveFromPlaylist()
			}
			return m, nil

		case key.Matches(msg, GlobalKeys.MoveDown):
			if m.viewType == ViewQueue {
				return m.handleQueueMoveDown()
			}
			return m, nil

		case key.Matches(msg, GlobalKeys.MoveUp):
			if m.viewType == ViewQueue {
				return m.handleQueueMoveUp()
			}
			return m, nil

		case key.Matches(msg, GlobalKeys.ShufflePlay):
			return m.handleShufflePlay()

		case key.Matches(msg, GlobalKeys.PlayNext):
			if m.viewType == ViewQueue {
				return m, nil
			}
			return m.handlePlayNext()

		case key.Matches(msg, GlobalKeys.AddTo):
			if m.viewType == ViewQueue || m.viewType == ViewPlaylistPicker {
				return m, nil
			}
			return m.handleAddToPlaylist()

		case key.Matches(msg, GlobalKeys.NewPlaylist):
			if m.activeTab == TabPlaylists && m.viewType == ViewPlaylists && len(m.navStack) == 0 {
				return m.handleNewPlaylist()
			}
			return m, nil

		case key.Matches(msg, GlobalKeys.SonosToggle):
			return m.handleSonosToggle()

		case key.Matches(msg, GlobalKeys.HelpToggle):
			m.showHelp = true
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab1):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabDiscover)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab2):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabArtists)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab3):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabAlbums)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab4):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabPlaylists)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab5):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabPodcasts)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab6):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabSearch)
			return m, nil

		case key.Matches(msg, GlobalKeys.Refresh):
			if m.viewType == ViewQueue {
				return m, nil
			}
			return m.refresh()
		}

		// Enter: drill down or play
		if msg.String() == "enter" {
			if m.viewType == ViewDiscover {
				return m.handleDiscoverEnter()
			}
			return m.handleEnter()
		}

		// Discover grid navigation
		if m.viewType == ViewDiscover {
			switch msg.String() {
			case "up", "k":
				return m.discoverMove(-1, 0)
			case "down", "j":
				return m.discoverMove(1, 0)
			case "left", "h":
				return m.discoverMove(0, -1)
			case "right", "l":
				return m.discoverMove(0, 1)
			}
			return m, nil
		}

		// Delegate to table
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case TickMsg:
		pos, total := m.pl.Progress()
		q := m.pl.Queue()
		m.bar.Update(m.pl.CurrentSong(), pos, total, m.pl.IsPaused(), m.pl.Volume(), m.pl.AudioInfo(), q.Index(), len(q.Songs()), q.Shuffle(), q.Repeat(), m.pl.IsSonosMode(), m.pl.SonosDeviceName())
		// Detect artist change for ListenBrainz popular-by-artist
		var cmds []tea.Cmd
		if song := m.pl.CurrentSong(); song != nil && song.Artist != m.lbCurrentArtist {
			m.lbCurrentArtist = song.Artist
			cmds = append(cmds, loadLBPopularCmd(m.lb, m.api, song.Artist, song.Title))
		}
		// In Sonos mode: poll transport state for queue advancement.
		if m.pl.IsSonosMode() && m.pl.CurrentSong() != nil {
			pl := m.pl
			cmds = append(cmds, func() tea.Msg {
				state, err := pl.SonosTransportState()
				if err == nil && state == "STOPPED" {
					return player.SongEndedMsg{}
				}
				return nil
			})
		}
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case player.SongEndedMsg:
		return m, m.nextTrack()

	case ArtistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading artists: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.artists = msg.Artists
		if m.activeTab == TabArtists && len(m.navStack) == 0 {
			m.setView(ViewArtists, m.artists)
		}
		return m, nil

	case AlbumListLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading albums: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.albumList = msg.Albums
		if m.activeTab == TabAlbums && len(m.navStack) == 0 {
			m.setView(ViewAlbums, m.albumList)
		}
		return m, nil

	case ArtistLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading artist: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Artist != nil {
			m.setView(ViewAlbums, msg.Artist.Albums)
		}
		return m, nil

	case AlbumLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading album: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Album != nil {
			m.setView(ViewSongs, msg.Album.Songs)
		}
		return m, nil

	case PlaylistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading playlists: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.playlists = msg.Playlists
		if m.activeTab == TabPlaylists && len(m.navStack) == 0 {
			m.setView(ViewPlaylists, m.playlists)
		}
		return m, nil

	case PlaylistLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading playlist: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Playlist != nil {
			m.setView(ViewSongs, msg.Playlist.Songs)
		}
		return m, nil

	case PlaylistCreatedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error creating playlist: %v", msg.Err), ToastError, 5*time.Second)
		}
		var cmds []tea.Cmd
		cmds = append(cmds, m.showToast("Playlist created", ToastSuccess, 2*time.Second))
		if msg.Playlist != nil && len(m.pendingSongIDs) > 0 {
			ids := m.pendingSongIDs
			m.pendingSongIDs = nil
			cmds = append(cmds, addToPlaylistCmd(m.api, msg.Playlist.ID, ids))
		}
		cmds = append(cmds, loadPlaylistsCmd(m.api))
		return m, tea.Batch(cmds...)

	case PlaylistUpdatedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error updating playlist: %v", msg.Err), ToastError, 5*time.Second)
		}
		return m, tea.Batch(
			loadPlaylistsCmd(m.api),
			m.showToast("Playlist updated", ToastSuccess, 2*time.Second),
		)

	case PlaylistDeletedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error deleting playlist: %v", msg.Err), ToastError, 5*time.Second)
		}
		var filtered []api.Playlist
		for _, p := range m.playlists {
			if p.ID != msg.ID {
				filtered = append(filtered, p)
			}
		}
		m.playlists = filtered
		if m.activeTab == TabPlaylists && len(m.navStack) == 0 {
			m.setView(ViewPlaylists, m.playlists)
		}
		return m, m.showToast("Playlist deleted", ToastSuccess, 2*time.Second)

	case PodcastsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading podcasts: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.podcasts = msg.Podcasts
		if m.activeTab == TabPodcasts && len(m.navStack) == 0 {
			m.setView(ViewPodcasts, m.podcasts)
		}
		return m, nil

	case PodcastLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading podcast: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Podcast != nil {
			m.setView(ViewEpisodes, msg.Podcast.Episodes)
		}
		return m, nil

	case SearchResultMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Search error: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Result != nil {
			items := buildSearchResults(msg.Result)
			m.setView(ViewSearchResults, items)
		}
		return m, nil

	case RandomSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading random songs: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case StarredLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading starred: %v", msg.Err), ToastError, 5*time.Second)
		}
		if msg.Result != nil {
			items := buildSearchResults(msg.Result)
			m.setView(ViewSearchResults, items)
		}
		return m, nil

	case GenresLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading genres: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.setView(ViewGenres, msg.Genres)
		return m, nil

	case SongsByGenreLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading songs by genre: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case SimilarSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading similar songs: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case TopSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading top songs: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case StarToggledMsg:
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Star error: %v", msg.Err), ToastError, 5*time.Second)
		}
		starVal := ""
		if msg.Starred {
			starVal = "starred"
		}
		// Update in viewData
		m.updateSongStarred(msg.ID, starVal)
		// Update in queue
		for i, s := range m.pl.Queue().Songs() {
			if s.ID == msg.ID {
				m.pl.Queue().Songs()[i].Starred = starVal
			}
		}
		// Update current song in player
		if cur := m.pl.CurrentSong(); cur != nil && cur.ID == msg.ID {
			cur.Starred = starVal
		}
		m.buildTable()
		label := "★ Starred"
		if !msg.Starred {
			label = "☆ Unstarred"
		}
		return m, m.showToast(label, ToastSuccess, 2*time.Second)

	case DiscoverDataLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Discover error: %v", msg.Err), ToastError, 5*time.Second)
		}
		m.discoverRecent = msg.Recent
		m.discoverNewest = msg.Newest
		m.discoverFrequent = msg.Frequent
		return m, nil

	case LBTrendingLoadedMsg:
		if msg.Err == nil {
			m.lbTrending = msg.Tracks
		}
		return m, nil

	case LBFreshReleasesLoadedMsg:
		if msg.Err == nil {
			m.lbFreshReleases = msg.Releases
		}
		return m, nil

	case LBPopularLoadedMsg:
		if msg.Err == nil {
			m.lbPopular = msg.Tracks
			m.lbPopularArtist = msg.ArtistName
		}
		return m, nil

	case LBPlaylistLoadedMsg:
		if msg.Err == nil && len(msg.Tracks) > 0 {
			switch msg.Kind {
			case "daily-jams":
				m.lbDailyJams = msg.Tracks
				m.lbDailyJamsName = msg.Name
			case "weekly-exploration":
				m.lbWeekly = msg.Tracks
				m.lbWeeklyName = msg.Name
			}
		}
		return m, nil

	case LBRecommendedLoadedMsg:
		if msg.Err == nil {
			m.lbRecommended = msg.Tracks
		}
		return m, nil

	case SonosDiscoveredMsg:
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Sonos discovery error: %v", msg.Err), ToastError, 4*time.Second)
		}
		if len(msg.Devices) == 0 {
			return m, m.showToast("No Sonos speakers found", ToastInfo, 3*time.Second)
		}
		m.sonosDevices = msg.Devices
		if len(msg.Devices) == 1 {
			return m.connectSonos(msg.Devices[0])
		}
		// Multiple speakers — show picker overlay.
		m.prevView = m.viewType
		m.prevViewData = m.viewData
		m.prevCursor = m.table.Cursor()
		m.viewType = ViewSonosPicker
		m.viewData = m.sonosDevices
		m.buildTable()
		return m, nil

	case AllSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading songs: %v", msg.Err), ToastError, 5*time.Second)
		}
		if len(msg.Songs) == 0 {
			return m, m.showToast("No songs found", ToastInfo, 2*time.Second)
		}
		m.pl.Queue().Set(msg.Songs, 0)
		if !m.pl.Queue().Shuffle() {
			m.pl.Queue().ToggleShuffle()
		}
		song := m.pl.Queue().Songs()[m.pl.Queue().Index()]
		streamURL := m.api.StreamURL(song.ID)
		return m, func() tea.Msg {
			if err := m.pl.Play(song, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			go m.api.Scrobble(song.ID) //nolint:errcheck
			return ShowToastMsg{Text: fmt.Sprintf("Shuffle playing %d songs", len(msg.Songs)), Level: ToastSuccess}
		}

	case ShowToastMsg:
		dur := 3 * time.Second
		if msg.Level == ToastError {
			dur = 5 * time.Second
		}
		return m, m.showToast(msg.Text, msg.Level, dur)

	case ToastExpiredMsg:
		if len(m.toasts) > 0 && m.toasts[0].id == msg.ID {
			m.toasts = m.toasts[:0]
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Catch-all: forward to table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Keep filtered results, dismiss search
		m.searching = false
		m.searchInput.Blur()
		m.searchData = nil
		m.relayout()
		return m, nil
	case "esc":
		// Restore original items
		m.searching = false
		m.searchInput.Blur()
		if m.searchData != nil {
			m.viewData = m.searchData
			m.buildTable()
			m.table.SetCursor(0)
		}
		m.searchData = nil
		m.relayout()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	// Live fuzzy filter
	m.filterTable()

	return m, cmd
}

func (m *Model) updateServerSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		query := m.serverInput.Value()
		if query == "" {
			return m, nil
		}
		m.serverSearching = false
		m.serverInput.Blur()
		m.loading = true
		return m, tea.Batch(searchCmd(m.api, query), m.spinner.Tick)
	case "esc":
		m.serverSearching = false
		m.serverInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.serverInput, cmd = m.serverInput.Update(msg)
	return m, cmd
}

func buildSearchResults(result *api.SearchResult) []SearchResultItem {
	var items []SearchResultItem
	for _, a := range result.Artists {
		items = append(items, SearchResultItem{Type: "artist", Artist: a})
	}
	for _, a := range result.Albums {
		items = append(items, SearchResultItem{Type: "album", Album: a})
	}
	for _, s := range result.Songs {
		items = append(items, SearchResultItem{Type: "song", Song: s})
	}
	return items
}

// filterTable applies fuzzy matching against the saved searchData.
func (m *Model) filterTable() {
	query := m.searchInput.Value()
	if query == "" {
		m.viewData = m.searchData
		m.buildTable()
		m.table.SetCursor(0)
		return
	}

	switch m.viewType {
	case ViewArtists:
		src := m.searchData.([]api.Artist)
		matches := fuzzy.FindFrom(query, &artistSource{src})
		filtered := make([]api.Artist, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewAlbums:
		src := m.searchData.([]api.Album)
		matches := fuzzy.FindFrom(query, &albumSource{src})
		filtered := make([]api.Album, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewPlaylists:
		src := m.searchData.([]api.Playlist)
		matches := fuzzy.FindFrom(query, &playlistSource{src})
		filtered := make([]api.Playlist, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewSongs:
		src := m.searchData.([]api.Song)
		matches := fuzzy.FindFrom(query, &songSource{src})
		filtered := make([]api.Song, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewPodcasts:
		src := m.searchData.([]api.Podcast)
		matches := fuzzy.FindFrom(query, &podcastSource{src})
		filtered := make([]api.Podcast, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewEpisodes:
		src := m.searchData.([]api.PodcastEpisode)
		matches := fuzzy.FindFrom(query, &episodeSource{src})
		filtered := make([]api.PodcastEpisode, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewSearchResults:
		src := m.searchData.([]SearchResultItem)
		matches := fuzzy.FindFrom(query, &searchResultSource{src})
		filtered := make([]SearchResultItem, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewDiscover:
		src := m.searchData.([]DiscoverOption)
		matches := fuzzy.FindFrom(query, &discoverSource{src})
		filtered := make([]DiscoverOption, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered

	case ViewGenres:
		src := m.searchData.([]api.Genre)
		matches := fuzzy.FindFrom(query, &genreSource{src})
		filtered := make([]api.Genre, len(matches))
		for i, match := range matches {
			filtered[i] = src[match.Index]
		}
		m.viewData = filtered
	}

	m.buildTable()
	m.table.SetCursor(0)
}

// Fuzzy search sources

type artistSource struct{ artists []api.Artist }

func (s *artistSource) String(i int) string { return s.artists[i].Name }
func (s *artistSource) Len() int            { return len(s.artists) }

type albumSource struct{ albums []api.Album }

func (s *albumSource) String(i int) string { return s.albums[i].Name + " " + s.albums[i].Artist }
func (s *albumSource) Len() int            { return len(s.albums) }

type playlistSource struct{ playlists []api.Playlist }

func (s *playlistSource) String(i int) string { return s.playlists[i].Name }
func (s *playlistSource) Len() int            { return len(s.playlists) }

type songSource struct{ songs []api.Song }

func (s *songSource) String(i int) string { return s.songs[i].Title + " " + s.songs[i].Artist }
func (s *songSource) Len() int            { return len(s.songs) }

type podcastSource struct{ podcasts []api.Podcast }

func (s *podcastSource) String(i int) string { return s.podcasts[i].Title }
func (s *podcastSource) Len() int            { return len(s.podcasts) }

type episodeSource struct{ episodes []api.PodcastEpisode }

func (s *episodeSource) String(i int) string { return s.episodes[i].Title }
func (s *episodeSource) Len() int            { return len(s.episodes) }

type discoverSource struct{ options []DiscoverOption }

func (s *discoverSource) String(i int) string { return s.options[i].Label }
func (s *discoverSource) Len() int            { return len(s.options) }

type genreSource struct{ genres []api.Genre }

func (s *genreSource) String(i int) string { return s.genres[i].Name }
func (s *genreSource) Len() int            { return len(s.genres) }

type searchResultSource struct{ items []SearchResultItem }

func (s *searchResultSource) String(i int) string {
	item := s.items[i]
	switch item.Type {
	case "artist":
		return item.Artist.Name
	case "album":
		return item.Album.Name + " " + item.Album.Artist
	case "song":
		return item.Song.Title + " " + item.Song.Artist
	}
	return ""
}
func (s *searchResultSource) Len() int { return len(s.items) }

func (m *Model) setView(vt ViewType, data any) {
	m.viewType = vt
	m.viewData = data
	m.buildTable()
	m.table.SetCursor(0)
}

func (m *Model) switchTab(tab Tab) {
	m.activeTab = tab
	m.navStack = nil
	m.currentPlaylistID = ""
	m.deleteConfirmID = ""
	m.serverSearching = false
	m.serverInput.Blur()
	if tab == TabSearch {
		m.serverSearching = true
		m.serverInput.SetValue("")
		m.serverInput.Focus()
		// Show empty results until user searches
		m.setView(ViewSearchResults, []SearchResultItem{})
		m.relayout()
		return
	}
	if tab == TabDiscover {
		m.setView(ViewDiscover, discoverOptions)
		m.relayout()
		return
	}
	m.populateView()
	m.relayout()
}

func (m *Model) populateView() {
	switch m.activeTab {
	case TabDiscover:
		m.setView(ViewDiscover, discoverOptions)
	case TabArtists:
		m.setView(ViewArtists, m.artists)
	case TabAlbums:
		m.setView(ViewAlbums, m.albumList)
	case TabPlaylists:
		m.setView(ViewPlaylists, m.playlists)
	case TabPodcasts:
		m.setView(ViewPodcasts, m.podcasts)
	case TabSearch:
		m.setView(ViewSearchResults, []SearchResultItem{})
	}
}

func (m *Model) refresh() (tea.Model, tea.Cmd) {
	m.loading = true
	switch m.activeTab {
	case TabDiscover:
		cmds := []tea.Cmd{
			loadDiscoverDataCmd(m.api), m.spinner.Tick,
			loadLBTrendingCmd(m.lb, m.api),
			loadLBFreshReleasesCmd(m.lb, m.api),
		}
		if song := m.pl.CurrentSong(); song != nil {
			cmds = append(cmds, loadLBPopularCmd(m.lb, m.api, song.Artist, song.Title))
		}
		if m.lb.HasUsername() {
			cmds = append(cmds,
				loadLBPlaylistCmd(m.lb, m.api, m.lb.Username(), "daily-jams"),
				loadLBPlaylistCmd(m.lb, m.api, m.lb.Username(), "weekly-exploration"),
			)
		}
		if m.lb.HasAuth() {
			cmds = append(cmds, loadLBRecommendedCmd(m.lb, m.api, m.lb.Username()))
		}
		return m, tea.Batch(cmds...)
	case TabArtists:
		return m, tea.Batch(loadArtistsCmd(m.api), m.spinner.Tick)
	case TabAlbums:
		return m, tea.Batch(loadAlbumListCmd(m.api), m.spinner.Tick)
	case TabPlaylists:
		return m, tea.Batch(loadPlaylistsCmd(m.api), m.spinner.Tick)
	case TabPodcasts:
		return m, tea.Batch(loadPodcastsCmd(m.api), m.spinner.Tick)
	}
	m.loading = false
	return m, nil
}

func (m *Model) pushNav(label string) {
	m.navStack = append(m.navStack, NavLevel{
		Label:    label,
		ViewType: m.viewType,
		Data:     m.viewData,
		Cursor:   m.table.Cursor(),
	})
}

func (m *Model) popNav() (tea.Model, tea.Cmd) {
	m.loading = false

	if len(m.navStack) == 0 {
		return m, nil
	}
	last := m.navStack[len(m.navStack)-1]
	m.navStack = m.navStack[:len(m.navStack)-1]

	if last.ViewType == ViewPlaylists {
		m.currentPlaylistID = ""
	}

	if last.Data != nil {
		m.viewType = last.ViewType
		m.viewData = last.Data
		m.buildTable()
		m.table.SetCursor(last.Cursor)
		return m, nil
	}

	// Top level — use cached data
	m.populateView()
	return m, nil
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	idx := m.table.Cursor()

	switch m.viewType {
	case ViewDiscover:
		options, _ := m.viewData.([]DiscoverOption)
		if idx < 0 || idx >= len(options) {
			return m, nil
		}
		opt := options[idx]
		switch opt.Action {
		case "random":
			m.pushNav("Random Songs")
			m.loading = true
			return m, tea.Batch(loadRandomSongsCmd(m.api), m.spinner.Tick)
		case "starred":
			m.pushNav("Starred")
			m.loading = true
			return m, tea.Batch(loadStarredCmd(m.api), m.spinner.Tick)
		case "genres":
			m.pushNav("By Genre")
			m.loading = true
			return m, tea.Batch(loadGenresCmd(m.api), m.spinner.Tick)
		case "similar":
			song := m.pl.CurrentSong()
			if song == nil {
				return m, m.showToast("No song currently playing", ToastError, 3*time.Second)
			}
			m.pushNav("Similar")
			m.loading = true
			return m, tea.Batch(loadSimilarSongsCmd(m.api, song.ID, song.ArtistID), m.spinner.Tick)
		case "top":
			song := m.pl.CurrentSong()
			if song == nil {
				return m, m.showToast("No song currently playing", ToastError, 3*time.Second)
			}
			m.pushNav("Top Songs")
			m.loading = true
			return m, tea.Batch(loadTopSongsCmd(m.api, song.Artist), m.spinner.Tick)
		}
		return m, nil

	case ViewGenres:
		genres, _ := m.viewData.([]api.Genre)
		if idx < 0 || idx >= len(genres) {
			return m, nil
		}
		g := genres[idx]
		m.pushNav(g.Name)
		m.loading = true
		return m, tea.Batch(loadSongsByGenreCmd(m.api, g.Name), m.spinner.Tick)

	case ViewArtists:
		artists := m.viewData.([]api.Artist)
		if idx < 0 || idx >= len(artists) {
			return m, nil
		}
		a := artists[idx]
		m.pushNav(a.Name)

		m.loading = true
		return m, tea.Batch(loadArtistCmd(m.api, a.ID), m.spinner.Tick)

	case ViewAlbums:
		albums := m.viewData.([]api.Album)
		if idx < 0 || idx >= len(albums) {
			return m, nil
		}
		a := albums[idx]
		m.pushNav(a.Name)

		m.loading = true
		return m, tea.Batch(loadAlbumCmd(m.api, a.ID), m.spinner.Tick)

	case ViewPlaylists:
		playlists := m.viewData.([]api.Playlist)
		if idx < 0 || idx >= len(playlists) {
			return m, nil
		}
		p := playlists[idx]
		m.currentPlaylistID = p.ID
		m.pushNav(p.Name)
		m.loading = true
		return m, tea.Batch(loadPlaylistCmd(m.api, p.ID), m.spinner.Tick)

	case ViewSongs:
		songs := m.viewData.([]api.Song)
		if idx < 0 || idx >= len(songs) {
			return m, nil
		}
		if songs[idx].ID == "" {
			return m, m.showToast("Not in library", ToastInfo, 2*time.Second)
		}
		return m, m.playSong(songs[idx])

	case ViewPodcasts:
		podcasts := m.viewData.([]api.Podcast)
		if idx < 0 || idx >= len(podcasts) {
			return m, nil
		}
		p := podcasts[idx]
		m.pushNav(p.Title)

		m.loading = true
		return m, tea.Batch(loadPodcastCmd(m.api, p.ID), m.spinner.Tick)

	case ViewEpisodes:
		episodes := m.viewData.([]api.PodcastEpisode)
		if idx < 0 || idx >= len(episodes) {
			return m, nil
		}
		return m, m.playEpisode(episodes[idx])

	case ViewSearchResults:
		items := m.viewData.([]SearchResultItem)
		if idx < 0 || idx >= len(items) {
			return m, nil
		}
		item := items[idx]
		switch item.Type {
		case "artist":
			m.pushNav(item.Artist.Name)
			m.loading = true
			return m, tea.Batch(loadArtistCmd(m.api, item.Artist.ID), m.spinner.Tick)
		case "album":
			m.pushNav(item.Album.Name)
			m.loading = true
			return m, tea.Batch(loadAlbumCmd(m.api, item.Album.ID), m.spinner.Tick)
		case "song":
			return m, m.playSongFromSearch(item.Song)
		}

	case ViewQueue:
		songs := m.pl.Queue().Songs()
		if idx < 0 || idx >= len(songs) {
			return m, nil
		}
		m.pl.Queue().SetIndex(idx)
		song := songs[idx]
		streamURL := m.api.StreamURL(song.ID)
		s := song
		return m, func() tea.Msg {
			if err := m.pl.Play(s, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			go m.api.Scrobble(s.ID) //nolint:errcheck
			return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", s.Title, s.Artist), Level: ToastSuccess}
		}

	}

	return m, nil
}

func (m *Model) playSong(song api.Song) tea.Cmd {
	allSongs := m.viewData.([]api.Song)
	// Filter out stub songs (no ID) when building queue
	var songs []api.Song
	var startIdx int
	for _, s := range allSongs {
		if s.ID == "" {
			continue
		}
		if s.ID == song.ID {
			startIdx = len(songs)
		}
		songs = append(songs, s)
	}
	m.pl.Queue().Set(songs, startIdx)

	streamURL := m.api.StreamURL(song.ID)
	s := song
	return func() tea.Msg {
		if err := m.pl.Play(s, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		go m.api.Scrobble(s.ID) //nolint:errcheck
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", s.Title, s.Artist), Level: ToastSuccess}
	}
}

func (m *Model) playSongFromSearch(song api.Song) tea.Cmd {
	// Build queue from all songs in search results
	items := m.viewData.([]SearchResultItem)
	var songs []api.Song
	var startIdx int
	for _, item := range items {
		if item.Type == "song" {
			if item.Song.ID == song.ID {
				startIdx = len(songs)
			}
			songs = append(songs, item.Song)
		}
	}
	if len(songs) > 0 {
		m.pl.Queue().Set(songs, startIdx)
	}

	streamURL := m.api.StreamURL(song.ID)
	s := song
	return func() tea.Msg {
		if err := m.pl.Play(s, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		go m.api.Scrobble(s.ID) //nolint:errcheck
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", s.Title, s.Artist), Level: ToastSuccess}
	}
}

func (m *Model) playEpisode(ep api.PodcastEpisode) tea.Cmd {
	// Convert episodes to songs for the queue
	episodes := m.viewData.([]api.PodcastEpisode)
	songs := make([]api.Song, 0, len(episodes))
	var startIdx int
	for _, e := range episodes {
		if e.StreamID == "" {
			continue
		}
		if e.ID == ep.ID {
			startIdx = len(songs)
		}
		songs = append(songs, api.Song{
			ID:       e.StreamID,
			Title:    e.Title,
			Duration: e.Duration,
			Suffix:   e.Suffix,
			BitRate:  e.BitRate,
		})
	}
	if len(songs) == 0 {
		return func() tea.Msg {
			return ShowToastMsg{Text: "Episode not available for streaming", Level: ToastError}
		}
	}
	m.pl.Queue().Set(songs, startIdx)

	song := songs[startIdx]
	streamURL := m.api.StreamURL(song.ID)
	return func() tea.Msg {
		if err := m.pl.Play(song, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s", song.Title), Level: ToastSuccess}
	}
}

func (m *Model) nextTrack() tea.Cmd {
	next, ok := m.pl.Queue().Next()
	if !ok {
		return nil
	}
	streamURL := m.api.StreamURL(next.ID)
	return func() tea.Msg {
		if err := m.pl.Play(next, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		go m.api.Scrobble(next.ID) //nolint:errcheck
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", next.Title, next.Artist), Level: ToastSuccess}
	}
}

func (m *Model) prevTrack() tea.Cmd {
	prev, ok := m.pl.Queue().Prev()
	if !ok {
		return nil
	}
	streamURL := m.api.StreamURL(prev.ID)
	return func() tea.Msg {
		if err := m.pl.Play(prev, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		go m.api.Scrobble(prev.ID) //nolint:errcheck
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", prev.Title, prev.Artist), Level: ToastSuccess}
	}
}

func (m *Model) showToast(text string, level ToastLevel, dur time.Duration) tea.Cmd {
	m.toastSeq++
	id := m.toastSeq
	m.toasts = []toast{{id: id, text: text, level: level}}
	return tea.Tick(dur, func(time.Time) tea.Msg {
		return ToastExpiredMsg{ID: id}
	})
}

func (m *Model) buildTable() {
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}

	var cols []table.Column
	var rows []table.Row

	switch m.viewType {
	case ViewDiscover:
		options, _ := m.viewData.([]DiscoverOption)
		numCols := 2
		padding := 2 * numCols
		descW := 40
		modeW := contentW - descW - padding
		if modeW < 10 {
			modeW = 10
		}
		cols = []table.Column{
			{Title: "Mode", Width: modeW},
			{Title: "Description", Width: descW},
		}
		rows = make([]table.Row, len(options))
		for i, o := range options {
			rows[i] = table.Row{o.Label, o.Description}
		}

	case ViewGenres:
		genres, _ := m.viewData.([]api.Genre)
		numCols := 3
		padding := 2 * numCols
		songsW := 8
		albumsW := 8
		fixedW := songsW + albumsW
		nameW := contentW - fixedW - padding
		if nameW < 10 {
			nameW = 10
		}
		cols = []table.Column{
			{Title: "Genre", Width: nameW},
			{Title: "Songs", Width: songsW},
			{Title: "Albums", Width: albumsW},
		}
		rows = make([]table.Row, len(genres))
		for i, g := range genres {
			rows[i] = table.Row{g.Name, fmt.Sprintf("%d", g.SongCount), fmt.Sprintf("%d", g.AlbumCount)}
		}

	case ViewArtists:
		artists, _ := m.viewData.([]api.Artist)
		numCols := 2
		padding := 2 * numCols
		albumsColW := 8
		nameW := contentW - albumsColW - padding
		if nameW < 10 {
			nameW = 10
		}
		cols = []table.Column{
			{Title: "Name", Width: nameW},
			{Title: "Albums", Width: albumsColW},
		}
		rows = make([]table.Row, len(artists))
		for i, a := range artists {
			rows[i] = table.Row{a.Name, fmt.Sprintf("%d", a.AlbumCount)}
		}

	case ViewAlbums:
		albums, _ := m.viewData.([]api.Album)
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
		cols = []table.Column{
			{Title: "Name", Width: nameW},
			{Title: "Artist", Width: artistW},
			{Title: "Year", Width: yearW},
			{Title: "Tracks", Width: tracksW},
		}
		rows = make([]table.Row, len(albums))
		for i, a := range albums {
			yearStr := ""
			if a.Year > 0 {
				yearStr = fmt.Sprintf("%d", a.Year)
			}
			rows[i] = table.Row{a.Name, a.Artist, yearStr, fmt.Sprintf("%d", a.SongCount)}
		}

	case ViewPlaylists, ViewPlaylistPicker:
		playlists, _ := m.viewData.([]api.Playlist)
		numCols := 2
		padding := 2 * numCols
		tracksW := 8
		nameW := contentW - tracksW - padding
		if nameW < 10 {
			nameW = 10
		}
		cols = []table.Column{
			{Title: "Name", Width: nameW},
			{Title: "Tracks", Width: tracksW},
		}
		rows = make([]table.Row, len(playlists))
		for i, p := range playlists {
			rows[i] = table.Row{p.Name, fmt.Sprintf("%d", p.SongCount)}
		}

	case ViewSongs:
		songs, _ := m.viewData.([]api.Song)
		showAlbum := m.activeTab == TabPlaylists
		dimStyle := lipgloss.NewStyle().Foreground(colorDimText)

		if showAlbum {
			numCols := 5
			padding := 2 * numCols
			trackW := 3
			timeW := 6
			fixedW := trackW + timeW
			remaining := contentW - fixedW - padding
			if remaining < 30 {
				remaining = 30
			}
			titleW := remaining * 35 / 100
			artistW := remaining * 30 / 100
			albumW := remaining - titleW - artistW
			cols = []table.Column{
				{Title: "#", Width: trackW},
				{Title: "Title", Width: titleW},
				{Title: "Artist", Width: artistW},
				{Title: "Album", Width: albumW},
				{Title: "Time", Width: timeW},
			}
			rows = make([]table.Row, len(songs))
			for i, s := range songs {
				if s.ID == "" {
					rows[i] = table.Row{
						dimStyle.Render("✗"),
						dimStyle.Render(s.Title),
						dimStyle.Render(s.Artist),
						dimStyle.Render(s.Album),
						"",
					}
					continue
				}
				durStr := fmt.Sprintf("%d:%02d", s.Duration/60, s.Duration%60)
				rows[i] = table.Row{
					formatTrackNumber(s.Track, s.DiscNumber),
					s.Title,
					s.Artist,
					s.Album,
					durStr,
				}
			}
		} else {
			numCols := 5
			padding := 2 * numCols
			trackW := 5
			timeW := 6
			kbpsW := 5
			fixedW := trackW + timeW + kbpsW
			remaining := contentW - fixedW - padding
			if remaining < 20 {
				remaining = 20
			}
			titleW := remaining * 55 / 100
			artistW := remaining - titleW
			cols = []table.Column{
				{Title: "#", Width: trackW},
				{Title: "Title", Width: titleW},
				{Title: "Artist", Width: artistW},
				{Title: "Time", Width: timeW},
				{Title: "Kbps", Width: kbpsW},
			}
			rows = make([]table.Row, len(songs))
			for i, s := range songs {
				trackStr := formatTrackNumber(s.Track, s.DiscNumber)
				durStr := fmt.Sprintf("%d:%02d", s.Duration/60, s.Duration%60)
				bitrateStr := ""
				if s.BitRate > 0 {
					bitrateStr = fmt.Sprintf("%d", s.BitRate)
				}
				if s.ID == "" {
					rows[i] = table.Row{
						dimStyle.Render("✗"),
						dimStyle.Render(s.Title),
						dimStyle.Render(s.Artist),
						"",
						"",
					}
					continue
				}
				rows[i] = table.Row{
					trackStr,
					s.Title,
					s.Artist,
					durStr,
					bitrateStr,
				}
			}
		}

	case ViewPodcasts:
		podcasts, _ := m.viewData.([]api.Podcast)
		numCols := 2
		padding := 2 * numCols
		epsW := 10
		nameW := contentW - epsW - padding
		if nameW < 10 {
			nameW = 10
		}
		cols = []table.Column{
			{Title: "Podcast", Width: nameW},
			{Title: "Episodes", Width: epsW},
		}
		rows = make([]table.Row, len(podcasts))
		for i, p := range podcasts {
			available := 0
			for _, ep := range p.Episodes {
				if ep.Status == "completed" {
					available++
				}
			}
			rows[i] = table.Row{p.Title, fmt.Sprintf("%d/%d", available, len(p.Episodes))}
		}

	case ViewEpisodes:
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
		cols = []table.Column{
			{Title: "Episode", Width: nameW},
			{Title: "Duration", Width: durW},
			{Title: "Status", Width: statusW},
		}
		rows = make([]table.Row, len(episodes))
		for i, e := range episodes {
			durStr := ""
			if e.Duration > 0 {
				durStr = formatDurationCompact(e.Duration)
			}
			rows[i] = table.Row{e.Title, durStr, e.Status}
		}

	case ViewSearchResults:
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
		cols = []table.Column{
			{Title: "Type", Width: typeW},
			{Title: "Name", Width: nameW},
			{Title: "Detail", Width: detailW},
		}
		rows = make([]table.Row, len(items))
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

	case ViewQueue:
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
		cols = []table.Column{
			{Title: "", Width: markerW},
			{Title: "#", Width: trackW},
			{Title: "Title", Width: titleW},
			{Title: "Artist", Width: artistW},
			{Title: "Time", Width: timeW},
		}
		rows = make([]table.Row, len(songs))
		for i, s := range songs {
			marker := " "
			if i == queueIdx {
				marker = "▶"
			}
			durStr := fmt.Sprintf("%d:%02d", s.Duration/60, s.Duration%60)
			rows[i] = table.Row{
				marker,
				fmt.Sprintf("%d", i+1),
				s.Title,
				s.Artist,
				durStr,
			}
		}

	case ViewSonosPicker:
		devices, _ := m.viewData.([]sonos.Device)
		nameW := contentW - 4
		if nameW < 10 {
			nameW = 10
		}
		cols = []table.Column{
			{Title: "Sonos Speaker", Width: nameW},
		}
		rows = make([]table.Row, len(devices))
		for i, d := range devices {
			rows[i] = table.Row{d.Name}
		}

	}

	// Clear rows first so UpdateViewport (triggered by SetColumns) doesn't
	// render old rows against the new column layout.
	m.table.SetRows(nil)
	m.table.SetColumns(cols)
	m.table.SetRows(rows)
}

func (m *Model) relayout() {
	headerH := 3
	breadcrumbH := 1
	playerH := 5
	contentBorder := 2
	helpH := 1
	contentHeight := m.height - headerH - breadcrumbH - playerH - contentBorder - helpH

	// Subtract lines for any input widgets rendered above the table.
	if m.activeTab == TabSearch || m.serverSearching {
		contentHeight-- // server search input
	}
	if m.searching {
		contentHeight-- // local filter input
	}
	if m.creatingNew {
		contentHeight-- // new playlist name input
	}

	if contentHeight < 1 {
		contentHeight = 1
	}
	contentW := m.width - 4
	if contentW < 1 {
		contentW = 1
	}
	m.table.SetWidth(contentW)
	m.table.SetHeight(contentHeight)
	m.bar.SetWidth(m.width)

	if m.viewData != nil {
		m.buildTable()
	}
}

func (m Model) View() tea.View {
	if !m.ready {
		return tea.View{Content: "Loading...", AltScreen: true}
	}

	cardW := m.width

	// Header card (title + tabs only)
	navW := cardW - 4
	if navW < 1 {
		navW = 1
	}
	header := CardStyle.Width(cardW).Render(m.renderNavBar(navW))

	// Breadcrumb line between header and content
	breadcrumbLeft := " " + m.renderBreadcrumb()
	breadcrumbRight := m.renderRowCounter()
	breadcrumb := padLine(breadcrumbLeft, breadcrumbRight, cardW)

	// Content dimensions for spinner placement
	headerH := 3
	breadcrumbH := 1
	playerH := 5
	contentBorder := 2
	helpH := 1
	contentHeight := m.height - headerH - breadcrumbH - playerH - contentBorder - helpH
	if contentHeight < 1 {
		contentHeight = 1
	}
	contentW := cardW - 4
	if contentW < 1 {
		contentW = 1
	}

	// Content card
	var contentParts []string
	if m.loading {
		spinnerView := m.spinner.View() + " Loading..."
		contentParts = append(contentParts, lipgloss.Place(contentW, contentHeight, lipgloss.Center, lipgloss.Center, spinnerView))
	} else {
		if m.serverSearching || m.activeTab == TabSearch {
			contentParts = append(contentParts, m.serverInput.View())
		}
		if m.searching {
			contentParts = append(contentParts, m.searchInput.View())
		}
		if m.creatingNew {
			contentParts = append(contentParts, m.pickerInput.View())
		}
		if m.viewType == ViewDiscover {
			contentParts = append(contentParts, m.renderDiscover(contentW, contentHeight))
		} else {
			contentParts = append(contentParts, m.table.View())
		}
	}

	contentInner := lipgloss.JoinVertical(lipgloss.Left, contentParts...)
	content := CardStyle.Width(cardW).Render(contentInner)

	// Player card + help line
	player := m.bar.View()
	helpLine := m.renderHelp()

	main := lipgloss.JoinVertical(lipgloss.Left, header, breadcrumb, content, player, helpLine)

	// Overlay: help popup (centered) or toast (top-right).
	layers := []*lipgloss.Layer{lipgloss.NewLayer(main).X(0).Y(0).Z(0)}
	if m.showHelp {
		popup := m.renderHelpPopup()
		popupW := lipgloss.Width(popup)
		popupH := lipgloss.Height(popup)
		x := (m.width - popupW) / 2
		y := (m.height - popupH) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		layers = append(layers, lipgloss.NewLayer(popup).X(x).Y(y).Z(10))
	} else if popup := m.renderToastPopup(); popup != "" {
		popupW := lipgloss.Width(popup)
		x := m.width - popupW - 1
		if x < 0 {
			x = 0
		}
		layers = append(layers, lipgloss.NewLayer(popup).X(x).Y(1).Z(10))
	}
	if len(layers) > 1 {
		comp := lipgloss.NewCompositor(layers...)
		canvas := lipgloss.NewCanvas(m.width, m.height)
		canvas.Compose(comp)
		return tea.View{Content: canvas.Render(), AltScreen: true}
	}

	return tea.View{Content: main, AltScreen: true}
}

func (m *Model) renderDiscoverStats() string {
	sep := SubtextStyle.Render(" · ")
	var parts []string
	if len(m.artists) > 0 {
		parts = append(parts, SubtextStyle.Render(fmt.Sprintf("%d artists", len(m.artists))))
	}
	if len(m.albumList) > 0 {
		parts = append(parts, SubtextStyle.Render(fmt.Sprintf("%d albums", len(m.albumList))))
	}
	if len(m.playlists) > 0 {
		parts = append(parts, SubtextStyle.Render(fmt.Sprintf("%d playlists", len(m.playlists))))
	}
	info := m.api.ServerInfo()
	if info.ServerVersion != "" {
		label := info.ServerVersion
		if info.Type != "" {
			label = info.Type + " " + label
		}
		parts = append(parts, SubtextStyle.Render(label))
	}
	if info.Version != "" {
		parts = append(parts, SubtextStyle.Render("API "+info.Version))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, sep) + " "
}

func (m *Model) renderToastPopup() string {
	if len(m.toasts) == 0 {
		return ""
	}
	t := m.toasts[0]
	var borderStyle lipgloss.Style
	var textStyle lipgloss.Style
	switch t.level {
	case ToastSuccess:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPlaying).Padding(0, 1)
		textStyle = ToastSuccessStyle
	case ToastError:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorError).Padding(0, 1)
		textStyle = ToastErrorStyle
	default:
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorFocused).Padding(0, 1)
		textStyle = ToastInfoStyle
	}
	return borderStyle.Render(textStyle.Render(t.text))
}

func (m *Model) renderNavBar(contentW int) string {
	title := TitleBarStyle.Render("🍸 subtonic")

	tabs := []struct {
		label string
		tab   Tab
	}{
		{"1 Discover", TabDiscover},
		{"2 Artists", TabArtists},
		{"3 Albums", TabAlbums},
		{"4 Playlists", TabPlaylists},
		{"5 Podcasts", TabPodcasts},
		{"6 Search", TabSearch},
	}

	parts := make([]string, len(tabs))
	for i, t := range tabs {
		if t.tab == m.activeTab {
			parts[i] = NavTabActiveStyle.Render(t.label)
		} else {
			parts[i] = NavTabStyle.Render(t.label)
		}
	}
	tabBar := strings.Join(parts, " ")

	gap := contentW - lipgloss.Width(title) - lipgloss.Width(tabBar)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + tabBar
}

func (m *Model) renderBreadcrumb() string {
	tabNames := map[Tab]string{
		TabDiscover:  "Discover",
		TabArtists:   "Artists",
		TabAlbums:    "Albums",
		TabPlaylists: "Playlists",
		TabPodcasts:  "Podcasts",
		TabSearch:    "Search",
	}

	parts := []string{tabNames[m.activeTab]}
	for _, level := range m.navStack {
		parts = append(parts, level.Label)
	}
	if m.viewType == ViewQueue {
		parts = append(parts, "Queue")
	}
	if m.viewType == ViewPlaylistPicker {
		parts = append(parts, "Add to Playlist")
	}

	return BreadcrumbStyle.Render(strings.Join(parts, " › "))
}

func (m *Model) renderRowCounter() string {
	if m.viewType == ViewDiscover {
		return m.renderDiscoverStats()
	}
	totalRows := len(m.table.Rows())
	if totalRows == 0 {
		return ""
	}
	cursor := m.table.Cursor() + 1
	counter := fmt.Sprintf("%d of %d", cursor, totalRows)

	visibleRows := m.table.Height() - 1
	if totalRows <= visibleRows {
		return PlayerMetaStyle.Render(counter + " ")
	}

	// Mini scrollbar
	barWidth := 10
	pct := float64(cursor-1) / float64(totalRows-1)
	thumbPos := int(pct * float64(barWidth-1))

	trackStyle := lipgloss.NewStyle().Foreground(colorDimText)
	thumbStyle := lipgloss.NewStyle().Foreground(colorFocused)

	var bar string
	for i := 0; i < barWidth; i++ {
		if i == thumbPos {
			bar += thumbStyle.Render("█")
		} else {
			bar += trackStyle.Render("░")
		}
	}

	return PlayerMetaStyle.Render(counter) + " " + bar + " "
}

// --- Star handling ---

// --- Discover navigation ---

func (m *Model) discoverMove(dSection, dItem int) (tea.Model, tea.Cmd) {
	secs := m.discoverSections()
	if len(secs) == 0 {
		return m, nil
	}
	m.discoverSection += dSection
	if m.discoverSection < 0 {
		m.discoverSection = 0
	}
	if m.discoverSection >= len(secs) {
		m.discoverSection = len(secs) - 1
	}
	sec := secs[m.discoverSection]
	m.discoverItem += dItem
	if m.discoverItem < 0 {
		m.discoverItem = 0
	}
	if m.discoverItem >= sec.count {
		m.discoverItem = sec.count - 1
	}
	return m, nil
}

func (m *Model) handleDiscoverEnter() (tea.Model, tea.Cmd) {
	secs := m.discoverSections()
	if m.discoverSection >= len(secs) {
		return m, nil
	}
	sec := secs[m.discoverSection]

	switch sec.kind {
	case secQuickActions:
		if m.discoverItem >= len(discoverOptions) {
			return m, nil
		}
		opt := discoverOptions[m.discoverItem]
		switch opt.Action {
		case "random":
			m.pushNav("Random Songs")
			m.loading = true
			return m, tea.Batch(loadRandomSongsCmd(m.api), m.spinner.Tick)
		case "starred":
			m.pushNav("Starred")
			m.loading = true
			return m, tea.Batch(loadStarredCmd(m.api), m.spinner.Tick)
		case "genres":
			m.pushNav("By Genre")
			m.loading = true
			return m, tea.Batch(loadGenresCmd(m.api), m.spinner.Tick)
		case "similar":
			song := m.pl.CurrentSong()
			if song == nil {
				return m, m.showToast("No song currently playing", ToastError, 3*time.Second)
			}
			m.pushNav("Similar")
			m.loading = true
			return m, tea.Batch(loadSimilarSongsCmd(m.api, song.ID, song.ArtistID), m.spinner.Tick)
		case "top":
			song := m.pl.CurrentSong()
			if song == nil {
				return m, m.showToast("No song currently playing", ToastError, 3*time.Second)
			}
			m.pushNav("Top Songs")
			m.loading = true
			return m, tea.Batch(loadTopSongsCmd(m.api, song.Artist), m.spinner.Tick)
		}

	case secRecent:
		if m.discoverItem < len(m.discoverRecent) {
			a := m.discoverRecent[m.discoverItem]
			m.pushNav(a.Name)
			m.loading = true
			return m, tea.Batch(loadAlbumCmd(m.api, a.ID), m.spinner.Tick)
		}

	case secNewest:
		if m.discoverItem < len(m.discoverNewest) {
			a := m.discoverNewest[m.discoverItem]
			m.pushNav(a.Name)
			m.loading = true
			return m, tea.Batch(loadAlbumCmd(m.api, a.ID), m.spinner.Tick)
		}

	case secFrequent:
		if m.discoverItem < len(m.discoverFrequent) {
			a := m.discoverFrequent[m.discoverItem]
			m.pushNav(a.Name)
			m.loading = true
			return m, tea.Batch(loadAlbumCmd(m.api, a.ID), m.spinner.Tick)
		}

	case secLBTrending, secLBPopular, secLBDailyJams, secLBWeekly, secLBRecommended:
		var tracks []DiscoverTrack
		switch sec.kind {
		case secLBTrending:
			tracks = m.lbTrending
		case secLBPopular:
			tracks = m.lbPopular
		case secLBDailyJams:
			tracks = m.lbDailyJams
		case secLBWeekly:
			tracks = m.lbWeekly
		case secLBRecommended:
			tracks = m.lbRecommended
		}
		if m.discoverItem >= len(tracks) {
			return m, nil
		}
		dt := tracks[m.discoverItem]
		if !dt.Available {
			return m, m.showToast("Not in library", ToastInfo, 2*time.Second)
		}
		// Queue all available tracks from section and play selected
		var songs []api.Song
		var startIdx int
		for _, t := range tracks {
			if t.Available {
				if t.Song.ID == dt.Song.ID {
					startIdx = len(songs)
				}
				songs = append(songs, t.Song)
			}
		}
		if len(songs) == 0 {
			return m, nil
		}
		m.pl.Queue().Set(songs, startIdx)
		song := dt.Song
		streamURL := m.api.StreamURL(song.ID)
		return m, func() tea.Msg {
			if err := m.pl.Play(song, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			go m.api.Scrobble(song.ID) //nolint:errcheck
			return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", song.Title, song.Artist), Level: ToastSuccess}
		}

	case secLBFreshReleases:
		if m.discoverItem >= len(m.lbFreshReleases) {
			return m, nil
		}
		dr := m.lbFreshReleases[m.discoverItem]
		if !dr.Available {
			return m, m.showToast("Not in library", ToastInfo, 2*time.Second)
		}
		m.pushNav(dr.Album.Name)
		m.loading = true
		return m, tea.Batch(loadAlbumCmd(m.api, dr.Album.ID), m.spinner.Tick)
	}

	return m, nil
}

func (m *Model) handlePlayNext() (tea.Model, tea.Cmd) {
	song := m.getTargetSong()
	if song == nil {
		return m, m.showToast("No song selected", ToastError, 2*time.Second)
	}
	if len(m.pl.Queue().Songs()) == 0 {
		return m, m.showToast("Queue is empty — play something first", ToastInfo, 2*time.Second)
	}
	m.pl.Queue().InsertNext(*song)
	return m, m.showToast(fmt.Sprintf("Playing next: %s", song.Title), ToastSuccess, 2*time.Second)
}

// --- Star handling ---

func (m *Model) handleStar() (tea.Model, tea.Cmd) {
	song := m.getTargetSong()
	if song == nil {
		return m, m.showToast("No song to star", ToastError, 2*time.Second)
	}
	return m, starToggleCmd(m.api, song.ID, song.Starred != "")
}

func (m *Model) getTargetSong() *api.Song {
	idx := m.table.Cursor()
	switch m.viewType {
	case ViewSongs:
		songs, _ := m.viewData.([]api.Song)
		if idx >= 0 && idx < len(songs) {
			return &songs[idx]
		}
	case ViewSearchResults:
		items, _ := m.viewData.([]SearchResultItem)
		if idx >= 0 && idx < len(items) && items[idx].Type == "song" {
			return &items[idx].Song
		}
	case ViewQueue:
		songs := m.pl.Queue().Songs()
		if idx >= 0 && idx < len(songs) {
			return &songs[idx]
		}
	}
	// Fallback: currently playing song
	return m.pl.CurrentSong()
}

func (m *Model) updateSongStarred(id, starVal string) {
	switch m.viewType {
	case ViewSongs:
		if songs, ok := m.viewData.([]api.Song); ok {
			for i := range songs {
				if songs[i].ID == id {
					songs[i].Starred = starVal
				}
			}
			m.viewData = songs
		}
	case ViewSearchResults:
		if items, ok := m.viewData.([]SearchResultItem); ok {
			for i := range items {
				if items[i].Type == "song" && items[i].Song.ID == id {
					items[i].Song.Starred = starVal
				}
			}
			m.viewData = items
		}
	}
}

// --- Queue view ---

func (m *Model) handleShufflePlay() (tea.Model, tea.Cmd) {
	idx := m.table.Cursor()
	switch m.viewType {
	case ViewArtists:
		artists, _ := m.viewData.([]api.Artist)
		if idx < 0 || idx >= len(artists) {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(loadArtistAllSongsCmd(m.api, artists[idx].ID), m.spinner.Tick)

	case ViewAlbums:
		albums, _ := m.viewData.([]api.Album)
		if len(albums) == 0 {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(loadAlbumsAllSongsCmd(m.api, albums), m.spinner.Tick)

	case ViewPlaylists:
		playlists, _ := m.viewData.([]api.Playlist)
		if idx < 0 || idx >= len(playlists) {
			return m, nil
		}
		pl := playlists[idx]
		m.loading = true
		return m, tea.Batch(loadPlaylistShuffleCmd(m.api, pl.ID), m.spinner.Tick)

	case ViewSongs:
		songs, _ := m.viewData.([]api.Song)
		if len(songs) == 0 {
			return m, nil
		}
		m.pl.Queue().Set(songs, 0)
		if !m.pl.Queue().Shuffle() {
			m.pl.Queue().ToggleShuffle()
		}
		song := m.pl.Queue().Songs()[m.pl.Queue().Index()]
		streamURL := m.api.StreamURL(song.ID)
		return m, func() tea.Msg {
			if err := m.pl.Play(song, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			go m.api.Scrobble(song.ID) //nolint:errcheck
			return ShowToastMsg{Text: fmt.Sprintf("Shuffle playing %d songs", len(songs)), Level: ToastSuccess}
		}
	}
	return m, nil
}

func (m *Model) handleOpenQueue() (tea.Model, tea.Cmd) {
	songs := m.pl.Queue().Songs()
	if len(songs) == 0 {
		return m, m.showToast("Queue is empty", ToastInfo, 2*time.Second)
	}
	m.prevView = m.viewType
	m.prevViewData = m.viewData
	m.prevCursor = m.table.Cursor()
	m.viewType = ViewQueue
	m.viewData = nil // queue reads directly from player
	m.buildTable()
	// Set cursor to currently playing song
	m.table.SetCursor(m.pl.Queue().Index())
	return m, nil
}

func (m *Model) closeOverlay() (tea.Model, tea.Cmd) {
	m.closeOverlaySilent()
	return m, nil
}

func (m *Model) closeOverlaySilent() {
	m.viewType = m.prevView
	m.viewData = m.prevViewData
	m.buildTable()
	m.table.SetCursor(m.prevCursor)
}

func (m *Model) handleQueueRemove() (tea.Model, tea.Cmd) {
	idx := m.table.Cursor()
	songs := m.pl.Queue().Songs()
	if idx < 0 || idx >= len(songs) {
		return m, nil
	}
	m.pl.Queue().Remove(idx)
	if len(m.pl.Queue().Songs()) == 0 {
		return m.closeOverlay()
	}
	m.buildTable()
	if idx >= len(m.pl.Queue().Songs()) {
		m.table.SetCursor(len(m.pl.Queue().Songs()) - 1)
	}
	return m, m.showToast("Removed from queue", ToastInfo, 2*time.Second)
}

func (m *Model) handleQueueMoveDown() (tea.Model, tea.Cmd) {
	idx := m.table.Cursor()
	songs := m.pl.Queue().Songs()
	if idx < 0 || idx >= len(songs)-1 {
		return m, nil
	}
	m.pl.Queue().Move(idx, idx+1)
	m.buildTable()
	m.table.SetCursor(idx + 1)
	return m, nil
}

func (m *Model) handleQueueMoveUp() (tea.Model, tea.Cmd) {
	idx := m.table.Cursor()
	if idx <= 0 {
		return m, nil
	}
	m.pl.Queue().Move(idx, idx-1)
	m.buildTable()
	m.table.SetCursor(idx - 1)
	return m, nil
}

// --- Playlist management ---

func (m *Model) updateCreatePlaylist(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := m.pickerInput.Value()
		if name == "" {
			return m, nil
		}
		m.creatingNew = false
		m.pickerInput.Blur()
		if m.viewType == ViewPlaylistPicker {
			m.closeOverlaySilent()
		}
		m.loading = true
		return m, tea.Batch(createPlaylistCmd(m.api, name), m.spinner.Tick)
	case "esc":
		m.creatingNew = false
		m.pickerInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.pickerInput, cmd = m.pickerInput.Update(msg)
	return m, cmd
}

func (m *Model) updatePlaylistPicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "enter":
		playlists, _ := m.viewData.([]api.Playlist)
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(playlists) {
			return m, nil
		}
		p := playlists[idx]
		ids := m.pendingSongIDs
		m.pendingSongIDs = nil
		m.closeOverlaySilent()
		return m, addToPlaylistCmd(m.api, p.ID, ids)
	case msg.String() == "n":
		m.creatingNew = true
		m.pickerInput.SetValue("")
		m.pickerInput.Focus()
		return m, nil
	case key.Matches(msg, GlobalKeys.Quit), key.Matches(msg, GlobalKeys.Back):
		m.pendingSongIDs = nil
		return m.closeOverlay()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) handleAddToPlaylist() (tea.Model, tea.Cmd) {
	song := m.getTargetSong()
	if song == nil || song.ID == "" {
		return m, m.showToast("No song to add", ToastError, 2*time.Second)
	}
	m.pendingSongIDs = []string{song.ID}
	return m.openPlaylistPicker()
}

func (m *Model) openPlaylistPicker() (tea.Model, tea.Cmd) {
	m.prevView = m.viewType
	m.prevViewData = m.viewData
	m.prevCursor = m.table.Cursor()
	m.viewType = ViewPlaylistPicker
	m.viewData = m.playlists
	m.buildTable()
	m.table.SetCursor(0)
	return m, nil
}

func (m *Model) handleNewPlaylist() (tea.Model, tea.Cmd) {
	m.creatingNew = true
	m.pickerInput.SetValue("")
	m.pickerInput.Focus()
	return m, nil
}

func (m *Model) handleDeletePlaylist() (tea.Model, tea.Cmd) {
	playlists, _ := m.viewData.([]api.Playlist)
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(playlists) {
		return m, nil
	}
	p := playlists[idx]
	m.deleteConfirmID = p.ID
	return m, m.showToast(fmt.Sprintf("Press d again to delete '%s'", p.Name), ToastInfo, 3*time.Second)
}

func (m *Model) handleRemoveFromPlaylist() (tea.Model, tea.Cmd) {
	songs, _ := m.viewData.([]api.Song)
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(songs) {
		return m, nil
	}
	playlistID := m.currentPlaylistID
	songs = append(songs[:idx], songs[idx+1:]...)
	m.viewData = songs
	m.buildTable()
	if idx >= len(songs) && len(songs) > 0 {
		m.table.SetCursor(len(songs) - 1)
	}
	return m, tea.Batch(
		removeFromPlaylistCmd(m.api, playlistID, []int{idx}),
		m.showToast("Removed from playlist", ToastSuccess, 2*time.Second),
	)
}

func (m *Model) renderHelp() string {
	sep := PlayerHelpSepStyle.Render(" · ")
	var keys []string
	switch m.viewType {
	case ViewQueue:
		keys = []string{
			"enter play",
			"d remove",
			"J/K move",
			"esc close",
		}
	case ViewPlaylistPicker:
		keys = []string{
			"enter select",
			"n new",
			"esc cancel",
		}
	case ViewSonosPicker:
		keys = []string{
			"enter connect",
			"esc cancel",
		}
	default:
		switch m.viewType {
		case ViewArtists, ViewAlbums, ViewSongs, ViewPlaylists:
			keys = append(keys, "p shuffle play")
		}
		if m.viewType == ViewPlaylists && m.activeTab == TabPlaylists && len(m.navStack) == 0 {
			keys = append(keys, "n new", "d delete")
		}
		if m.viewType == ViewSongs && m.currentPlaylistID != "" {
			keys = append(keys, "d remove")
		}
		if m.viewType == ViewSongs || m.viewType == ViewSearchResults {
			keys = append(keys, "a add to playlist", "e play next")
		}
		keys = append(keys, "? help")
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = PlayerHelpKeyStyle.Render(k)
	}
	return " " + strings.Join(parts, sep)
}

// --- Help popup ---

func (m *Model) renderHelpPopup() string {
	type entry struct{ key, desc string }
	type section struct {
		title   string
		entries []entry
	}

	sections := []section{
		{
			"Playback",
			[]entry{
				{"space", "play / pause"},
				{"> or .", "next track"},
				{"< or ,", "previous track"},
				{"= / +", "volume +1 / +5"},
				{"- / _", "volume -1 / -5"},
				{"s", "toggle shuffle"},
				{"r", "cycle repeat"},
				{"o", "toggle Sonos output"},
			},
		},
		{
			"Navigation",
			[]entry{
				{"1 – 6", "switch tab"},
				{"/", "filter / search"},
				{"esc", "go back"},
				{"Q", "open queue"},
				{"R", "refresh"},
				{"?", "close this help"},
				{"q", "quit"},
			},
		},
	}

	// View-specific section
	var viewTitle string
	var viewEntries []entry
	switch m.viewType {
	case ViewSongs:
		viewTitle = "Songs"
		viewEntries = []entry{
			{"enter", "play from here"},
			{"p", "shuffle play all"},
			{"e", "play next"},
			{"S", "star / unstar"},
			{"a", "add to playlist"},
		}
		if m.currentPlaylistID != "" {
			viewEntries = append(viewEntries, entry{"d / del", "remove from playlist"})
		}
	case ViewQueue:
		viewTitle = "Queue"
		viewEntries = []entry{
			{"d / del", "remove from queue"},
			{"J", "move song down"},
			{"K", "move song up"},
		}
	case ViewArtists:
		viewTitle = "Artists"
		viewEntries = []entry{
			{"enter", "browse albums"},
			{"p", "shuffle play all"},
		}
	case ViewAlbums:
		viewTitle = "Albums"
		viewEntries = []entry{
			{"enter", "browse songs"},
			{"p", "shuffle play all"},
		}
	case ViewPlaylists:
		viewTitle = "Playlists"
		viewEntries = []entry{
			{"enter", "open playlist"},
			{"p", "shuffle play"},
			{"n", "new playlist"},
			{"d / del", "delete playlist"},
		}
	case ViewSearchResults:
		viewTitle = "Search Results"
		viewEntries = []entry{
			{"enter", "open / play"},
			{"S", "star / unstar"},
			{"a", "add to playlist"},
			{"e", "play next"},
		}
	case ViewGenres:
		viewTitle = "Genres"
		viewEntries = []entry{
			{"enter", "browse genre songs"},
		}
	case ViewDiscover:
		viewTitle = "Discover"
		viewEntries = []entry{
			{"enter", "explore section"},
			{"↑↓←→ / hjkl", "navigate grid"},
		}
	case ViewPodcasts:
		viewTitle = "Podcasts"
		viewEntries = []entry{
			{"enter", "open podcast"},
		}
	case ViewEpisodes:
		viewTitle = "Episodes"
		viewEntries = []entry{
			{"enter", "play episode"},
		}
	}
	if len(viewEntries) > 0 {
		sections = append(sections, section{viewTitle, viewEntries})
	}

	const keyColW = 14
	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, HelpSectionStyle.Render(sec.title))
		for _, e := range sec.entries {
			key := HelpKeyStyle.Width(keyColW).Render(e.key)
			desc := HelpDescStyle.Render(e.desc)
			lines = append(lines, "  "+key+desc)
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFocused).
		Padding(1, 2).
		Render(content)
}

// --- Sonos ---

func (m *Model) handleSonosToggle() (tea.Model, tea.Cmd) {
	if m.pl.IsSonosMode() {
		return m.disconnectSonos()
	}
	// Not in Sonos mode — discover or show existing devices.
	if len(m.sonosDevices) > 0 {
		if len(m.sonosDevices) == 1 {
			return m.connectSonos(m.sonosDevices[0])
		}
		m.prevView = m.viewType
		m.prevViewData = m.viewData
		m.prevCursor = m.table.Cursor()
		m.viewType = ViewSonosPicker
		m.viewData = m.sonosDevices
		m.buildTable()
		return m, nil
	}
	return m, tea.Batch(
		discoverSonosCmd(),
		m.showToast("Discovering Sonos speakers...", ToastInfo, 4*time.Second),
	)
}

func (m *Model) updateSonosPicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "enter":
		devices, _ := m.viewData.([]sonos.Device)
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(devices) {
			return m, nil
		}
		dev := devices[idx]
		m.closeOverlaySilent()
		return m.connectSonos(dev)
	case key.Matches(msg, GlobalKeys.Quit), key.Matches(msg, GlobalKeys.Back):
		return m.closeOverlay()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// connectSonos enables Sonos output on dev and resumes the current song from
// its current position on the new output.
func (m *Model) connectSonos(dev sonos.Device) (tea.Model, tea.Cmd) {
	song := m.pl.CurrentSong()
	pos, _ := m.pl.Progress()
	m.pl.EnableSonos(sonos.New(dev))
	if song != nil && song.ID != "" {
		s := *song
		streamURL := m.api.StreamURL(s.ID)
		pl := m.pl
		seekPos := pos
		return m, func() tea.Msg {
			if err := pl.Play(s, streamURL); err != nil {
				return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
			}
			pl.SeekTo(seekPos)
			return ShowToastMsg{Text: fmt.Sprintf("Connected to %s", dev.Name), Level: ToastSuccess}
		}
	}
	return m, m.showToast(fmt.Sprintf("Connected to %s", dev.Name), ToastSuccess, 3*time.Second)
}

// disconnectSonos disables Sonos output and resumes the current song on local
// audio from the same position.
func (m *Model) disconnectSonos() (tea.Model, tea.Cmd) {
	song := m.pl.CurrentSong()
	if song == nil || song.ID == "" {
		m.pl.DisableSonos()
		return m, m.showToast("Switched to local audio", ToastInfo, 2*time.Second)
	}
	s := *song
	streamURL := m.api.StreamURL(s.ID)
	pl := m.pl
	return m, func() tea.Msg {
		// Query Sonos directly for the current position — don't rely on the
		// 2-second background-poll cache which may be stale.
		freshPos := pl.FreshSonosPosition()
		pl.DisableSonos()
		if err := pl.PlayFrom(s, streamURL, freshPos); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		return ShowToastMsg{Text: "Switched to local audio", Level: ToastInfo}
	}
}
