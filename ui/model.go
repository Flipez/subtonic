package ui

import (
	"fmt"
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
	TabHome Tab = iota
	TabDiscover
	TabBrowse
	TabPlaylists
	TabPodcasts
	TabSearch
)

const (
	toastShort = 2 * time.Second
	toastLong  = 5 * time.Second
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
	ViewBrowse
	ViewHome
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
	Action      string // "random", "similar", "top"
}

var discoverOptions = []DiscoverOption{
	{Label: "Random Songs", Description: "Shuffle across entire library", Action: "random"},
	{Label: "Similar", Description: "Based on currently playing song", Action: "similar"},
	{Label: "Top Songs", Description: "Top tracks of currently playing artist", Action: "top"},
}

// BrowseOption represents a category in the Browse tab.
type BrowseOption struct {
	Label       string
	Description string
	Action      string // "artists", "albums", "genres", "starred"
}

var browseOptions = []BrowseOption{
	{Label: "Artists", Description: "Browse by artist", Action: "artists"},
	{Label: "Albums", Description: "Browse all albums", Action: "albums"},
	{Label: "By Genre", Description: "Pick a genre, get songs", Action: "genres"},
	{Label: "Starred", Description: "Favorited artists, albums, and songs", Action: "starred"},
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

	// Quick actions popup
	showQuickActions bool
	quickActionsIdx  int

	// Info popup (artist bio / album notes)
	showInfo    bool
	infoLoading bool
	infoTitle   string
	infoContent string
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
		activeTab:   TabHome,
		table:       t,
		searchInput: input,
		serverInput: serverInput,
		pickerInput: pickerInput,
		bar:         NewPlayerBar(0),
		spinner:     sp,
		viewType:    ViewHome,
		viewData:    nil,
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

		// Info popup: any key closes it.
		if m.showInfo {
			m.showInfo = false
			return m, nil
		}

		// Quick actions popup: navigable; x/esc closes, enter activates.
		if m.showQuickActions {
			return m.updateQuickActions(msg)
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
				return m.loadWithSpinner(deletePlaylistCmd(m.api, id))
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
			return m, m.showToast(label, ToastInfo, toastShort)

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
			return m, m.showToast(label, ToastInfo, toastShort)

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

		case key.Matches(msg, GlobalKeys.QuickActions):
			m.showQuickActions = !m.showQuickActions
			return m, nil

		case key.Matches(msg, GlobalKeys.Info):
			switch m.viewType {
			case ViewArtists:
				if artists, ok := m.viewData.([]api.Artist); ok {
					idx := m.table.Cursor()
					if idx >= 0 && idx < len(artists) {
						a := artists[idx]
						m.showInfo = true
						m.infoLoading = true
						m.infoTitle = a.Name
						m.infoContent = ""
						return m, tea.Batch(loadInfoCmd(m.api, "artist", a.ID, a.Name), m.spinner.Tick)
					}
				}
			case ViewAlbums:
				if albums, ok := m.viewData.([]api.Album); ok {
					idx := m.table.Cursor()
					if idx >= 0 && idx < len(albums) {
						al := albums[idx]
						m.showInfo = true
						m.infoLoading = true
						m.infoTitle = al.Name
						m.infoContent = ""
						return m, tea.Batch(loadInfoCmd(m.api, "album", al.ID, al.Name), m.spinner.Tick)
					}
				}
			}
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab1):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabHome)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab2):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabDiscover)
			return m, nil

		case key.Matches(msg, GlobalKeys.Tab3):
			if m.viewType == ViewQueue {
				return m, nil
			}
			m.switchTab(TabBrowse)
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
			if m.viewType == ViewDiscover || m.viewType == ViewHome {
				return m.handleDiscoverEnter()
			}
			return m.handleEnter()
		}

		// Home/Discover grid navigation
		if m.viewType == ViewDiscover || m.viewType == ViewHome {
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
		m.bar.Update(m.pl.CurrentSong(), pos, total, m.pl.IsPaused(), m.pl.Volume(), m.pl.AudioInfo(), q.Index(), len(q.Songs()), q.Shuffle(), q.Repeat(), m.pl.IsSonosMode(), m.pl.SonosGroupSize())
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
					return sonosSongEndedMsg{}
				}
				return nil
			})
		}
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case player.SongEndedMsg:
		return m, m.nextTrack()

	case sonosSongEndedMsg:
		// Only advance if still in Sonos mode — stale polls arrive after
		// switching back to local (DisableSonos calls Stop(), making in-flight
		// goroutines see "STOPPED" and return this message).
		if m.pl.IsSonosMode() {
			return m, m.nextTrack()
		}
		return m, nil

	case ArtistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading artists: %v", msg.Err), ToastError, toastLong)
		}
		m.artists = msg.Artists
		if m.viewType == ViewArtists {
			m.setView(ViewArtists, m.artists)
		}
		return m, nil

	case AlbumListLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading albums: %v", msg.Err), ToastError, toastLong)
		}
		m.albumList = msg.Albums
		if m.viewType == ViewAlbums {
			m.setView(ViewAlbums, m.albumList)
		}
		return m, nil

	case ArtistLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading artist: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Artist != nil {
			m.setView(ViewAlbums, msg.Artist.Albums)
		}
		return m, nil

	case AlbumLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading album: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Album != nil {
			m.setView(ViewSongs, msg.Album.Songs)
		}
		return m, nil

	case PlaylistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading playlists: %v", msg.Err), ToastError, toastLong)
		}
		m.playlists = msg.Playlists
		if m.activeTab == TabPlaylists && len(m.navStack) == 0 {
			m.setView(ViewPlaylists, m.playlists)
		}
		return m, nil

	case PlaylistLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading playlist: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Playlist != nil {
			m.setView(ViewSongs, msg.Playlist.Songs)
		}
		return m, nil

	case PlaylistCreatedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error creating playlist: %v", msg.Err), ToastError, toastLong)
		}
		var cmds []tea.Cmd
		cmds = append(cmds, m.showToast("Playlist created", ToastSuccess, toastShort))
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
			return m, m.showToast(fmt.Sprintf("Error updating playlist: %v", msg.Err), ToastError, toastLong)
		}
		return m, tea.Batch(
			loadPlaylistsCmd(m.api),
			m.showToast("Playlist updated", ToastSuccess, toastShort),
		)

	case PlaylistDeletedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error deleting playlist: %v", msg.Err), ToastError, toastLong)
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
		return m, m.showToast("Playlist deleted", ToastSuccess, toastShort)

	case PodcastsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading podcasts: %v", msg.Err), ToastError, toastLong)
		}
		m.podcasts = msg.Podcasts
		if m.activeTab == TabPodcasts && len(m.navStack) == 0 {
			m.setView(ViewPodcasts, m.podcasts)
		}
		return m, nil

	case PodcastLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading podcast: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Podcast != nil {
			m.setView(ViewEpisodes, msg.Podcast.Episodes)
		}
		return m, nil

	case SearchResultMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Search error: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Result != nil {
			items := buildSearchResults(msg.Result)
			m.setView(ViewSearchResults, items)
		}
		return m, nil

	case RandomSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading random songs: %v", msg.Err), ToastError, toastLong)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case StarredLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading starred: %v", msg.Err), ToastError, toastLong)
		}
		if msg.Result != nil {
			items := buildSearchResults(msg.Result)
			m.setView(ViewSearchResults, items)
		}
		return m, nil

	case GenresLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading genres: %v", msg.Err), ToastError, toastLong)
		}
		m.setView(ViewGenres, msg.Genres)
		return m, nil

	case SongsByGenreLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading songs by genre: %v", msg.Err), ToastError, toastLong)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case SimilarSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading similar songs: %v", msg.Err), ToastError, toastLong)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case InfoLoadedMsg:
		m.infoLoading = false
		if msg.Err != nil {
			m.showInfo = false
			return m, m.showToast(fmt.Sprintf("Error loading info: %v", msg.Err), ToastError, toastLong)
		}
		m.infoContent = msg.Content
		return m, nil

	case TopSongsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Error loading top songs: %v", msg.Err), ToastError, toastLong)
		}
		m.setView(ViewSongs, msg.Songs)
		return m, nil

	case StarToggledMsg:
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Star error: %v", msg.Err), ToastError, toastLong)
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
		label := IconHeart + " Starred"
		if !msg.Starred {
			label = IconHeartOutline + " Unstarred"
		}
		return m, m.showToast(label, ToastSuccess, toastShort)

	case DiscoverDataLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("Discover error: %v", msg.Err), ToastError, toastLong)
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
			return m, m.showToast(fmt.Sprintf("Sonos discovery error: %v", msg.Err), ToastError, toastLong)
		}
		if len(msg.Devices) == 0 {
			return m, m.showToast("No Sonos speakers found", ToastInfo, toastShort)
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
			return m, m.showToast(fmt.Sprintf("Error loading songs: %v", msg.Err), ToastError, toastLong)
		}
		if len(msg.Songs) == 0 {
			return m, m.showToast("No songs found", ToastInfo, toastShort)
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
		dur := toastShort
		if msg.Level == ToastError {
			dur = toastLong
		}
		return m, m.showToast(msg.Text, msg.Level, dur)

	case ToastExpiredMsg:
		if len(m.toasts) > 0 && m.toasts[0].id == msg.ID {
			m.toasts = m.toasts[:0]
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.infoLoading {
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
		return m.loadWithSpinner(searchCmd(m.api, query))
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
	case TabHome:
		m.setView(ViewHome, nil)
	case TabDiscover:
		m.setView(ViewDiscover, discoverOptions)
	case TabBrowse:
		m.setView(ViewBrowse, browseOptions)
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
	case TabHome:
		return m.loadWithSpinner(loadDiscoverDataCmd(m.api))
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
	case TabBrowse:
		return m.loadWithSpinner(tea.Batch(loadArtistsCmd(m.api), loadAlbumListCmd(m.api)))
	case TabPlaylists:
		return m.loadWithSpinner(loadPlaylistsCmd(m.api))
	case TabPodcasts:
		return m.loadWithSpinner(loadPodcastsCmd(m.api))
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
		return m.handleEnterDiscover(idx)
	case ViewGenres:
		return m.handleEnterGenres(idx)
	case ViewArtists:
		return m.handleEnterArtists(idx)
	case ViewAlbums:
		return m.handleEnterAlbums(idx)
	case ViewPlaylists:
		return m.handleEnterPlaylists(idx)
	case ViewSongs:
		return m.handleEnterSongs(idx)
	case ViewPodcasts:
		return m.handleEnterPodcasts(idx)
	case ViewEpisodes:
		return m.handleEnterEpisodes(idx)
	case ViewSearchResults:
		return m.handleEnterSearchResults(idx)
	case ViewQueue:
		return m.handleEnterQueue(idx)
	case ViewBrowse:
		return m.handleEnterBrowse(idx)
	}

	return m, nil
}

func (m *Model) handleEnterBrowse(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(browseOptions) {
		return m, nil
	}
	opt := browseOptions[idx]
	switch opt.Action {
	case "artists":
		m.pushNav("Artists")
		m.setView(ViewArtists, m.artists)
		if len(m.artists) == 0 {
			return m.loadWithSpinner(loadArtistsCmd(m.api))
		}
		return m, nil
	case "albums":
		m.pushNav("Albums")
		m.setView(ViewAlbums, m.albumList)
		if len(m.albumList) == 0 {
			return m.loadWithSpinner(loadAlbumListCmd(m.api))
		}
		return m, nil
	case "genres":
		m.pushNav("By Genre")
		return m.loadWithSpinner(loadGenresCmd(m.api))
	case "starred":
		m.pushNav("Starred")
		return m.loadWithSpinner(loadStarredCmd(m.api))
	}
	return m, nil
}

func (m *Model) loadWithSpinner(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.loading = true
	return m, tea.Batch(cmd, m.spinner.Tick)
}

func (m *Model) handleEnterDiscover(idx int) (tea.Model, tea.Cmd) {
	options, _ := m.viewData.([]DiscoverOption)
	if idx < 0 || idx >= len(options) {
		return m, nil
	}
	opt := options[idx]
	switch opt.Action {
	case "random":
		m.pushNav("Random Songs")
		return m.loadWithSpinner(loadRandomSongsCmd(m.api))
	case "starred":
		m.pushNav("Starred")
		return m.loadWithSpinner(loadStarredCmd(m.api))
	case "genres":
		m.pushNav("By Genre")
		return m.loadWithSpinner(loadGenresCmd(m.api))
	case "similar":
		song := m.pl.CurrentSong()
		if song == nil {
			return m, m.showToast("No song currently playing", ToastError, toastLong)
		}
		m.pushNav("Similar")
		return m.loadWithSpinner(loadSimilarSongsCmd(m.api, song.ID, song.ArtistID))
	case "top":
		song := m.pl.CurrentSong()
		if song == nil {
			return m, m.showToast("No song currently playing", ToastError, toastLong)
		}
		m.pushNav("Top Songs")
		return m.loadWithSpinner(loadTopSongsCmd(m.api, song.Artist))
	}
	return m, nil
}

func (m *Model) handleEnterGenres(idx int) (tea.Model, tea.Cmd) {
	genres, _ := m.viewData.([]api.Genre)
	if idx < 0 || idx >= len(genres) {
		return m, nil
	}
	genre := genres[idx]
	m.pushNav(genre.Name)
	return m.loadWithSpinner(loadSongsByGenreCmd(m.api, genre.Name))
}

func (m *Model) handleEnterArtists(idx int) (tea.Model, tea.Cmd) {
	artists := m.viewData.([]api.Artist)
	if idx < 0 || idx >= len(artists) {
		return m, nil
	}
	artist := artists[idx]
	m.pushNav(artist.Name)
	return m.loadWithSpinner(loadArtistCmd(m.api, artist.ID))
}

func (m *Model) handleEnterAlbums(idx int) (tea.Model, tea.Cmd) {
	albums := m.viewData.([]api.Album)
	if idx < 0 || idx >= len(albums) {
		return m, nil
	}
	album := albums[idx]
	m.pushNav(album.Name)
	return m.loadWithSpinner(loadAlbumCmd(m.api, album.ID))
}

func (m *Model) handleEnterPlaylists(idx int) (tea.Model, tea.Cmd) {
	playlists := m.viewData.([]api.Playlist)
	if idx < 0 || idx >= len(playlists) {
		return m, nil
	}
	playlist := playlists[idx]
	m.currentPlaylistID = playlist.ID
	m.pushNav(playlist.Name)
	return m.loadWithSpinner(loadPlaylistCmd(m.api, playlist.ID))
}

func (m *Model) handleEnterSongs(idx int) (tea.Model, tea.Cmd) {
	songs := m.viewData.([]api.Song)
	if idx < 0 || idx >= len(songs) {
		return m, nil
	}
	if songs[idx].ID == "" {
		return m, m.showToast("Not in library", ToastInfo, toastShort)
	}
	return m, m.playSong(songs[idx])
}

func (m *Model) handleEnterPodcasts(idx int) (tea.Model, tea.Cmd) {
	podcasts := m.viewData.([]api.Podcast)
	if idx < 0 || idx >= len(podcasts) {
		return m, nil
	}
	podcast := podcasts[idx]
	m.pushNav(podcast.Title)
	return m.loadWithSpinner(loadPodcastCmd(m.api, podcast.ID))
}

func (m *Model) handleEnterEpisodes(idx int) (tea.Model, tea.Cmd) {
	episodes := m.viewData.([]api.PodcastEpisode)
	if idx < 0 || idx >= len(episodes) {
		return m, nil
	}
	return m, m.playEpisode(episodes[idx])
}

func (m *Model) handleEnterSearchResults(idx int) (tea.Model, tea.Cmd) {
	items := m.viewData.([]SearchResultItem)
	if idx < 0 || idx >= len(items) {
		return m, nil
	}
	item := items[idx]
	switch item.Type {
	case "artist":
		m.pushNav(item.Artist.Name)
		return m.loadWithSpinner(loadArtistCmd(m.api, item.Artist.ID))
	case "album":
		m.pushNav(item.Album.Name)
		return m.loadWithSpinner(loadAlbumCmd(m.api, item.Album.ID))
	case "song":
		return m, m.playSongFromSearch(item.Song)
	}
	return m, nil
}

func (m *Model) handleEnterQueue(idx int) (tea.Model, tea.Cmd) {
	songs := m.pl.Queue().Songs()
	if idx < 0 || idx >= len(songs) {
		return m, nil
	}
	m.pl.Queue().SetIndex(idx)
	song := songs[idx]
	streamURL := m.api.StreamURL(song.ID)
	return m, func() tea.Msg {
		if err := m.pl.Play(song, streamURL); err != nil {
			return ShowToastMsg{Text: fmt.Sprintf("Play error: %v", err), Level: ToastError}
		}
		go m.api.Scrobble(song.ID) //nolint:errcheck
		return ShowToastMsg{Text: fmt.Sprintf("Now playing: %s — %s", song.Title, song.Artist), Level: ToastSuccess}
	}
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


func (m *Model) relayout() {
	headerH := 3
	breadcrumbH := 1
	playerH := 3
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
	playerH := 3
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
		if m.viewType == ViewHome {
			contentParts = append(contentParts, m.renderHome(contentW, contentHeight))
		} else if m.viewType == ViewDiscover {
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

	// Composite: main content + optional overlays.
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
	} else if m.showQuickActions {
		popup := m.renderQuickActionsPopup()
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
	} else if m.showInfo {
		popup := m.renderInfoPopup()
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
	comp := lipgloss.NewCompositor(layers...)
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(comp)
	return tea.View{Content: canvas.Render(), AltScreen: true}
}



// --- Star handling ---


func (m *Model) handlePlayNext() (tea.Model, tea.Cmd) {
	song := m.getTargetSong()
	if song == nil {
		return m, m.showToast("No song selected", ToastError, toastShort)
	}
	if len(m.pl.Queue().Songs()) == 0 {
		return m, m.showToast("Queue is empty — play something first", ToastInfo, toastShort)
	}
	m.pl.Queue().InsertNext(*song)
	return m, m.showToast(fmt.Sprintf("Playing next: %s", song.Title), ToastSuccess, toastShort)
}

// --- Star handling ---

func (m *Model) handleStar() (tea.Model, tea.Cmd) {
	song := m.getTargetSong()
	if song == nil {
		return m, m.showToast("No song to star", ToastError, toastShort)
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
		return m.loadWithSpinner(loadArtistAllSongsCmd(m.api, artists[idx].ID))

	case ViewAlbums:
		albums, _ := m.viewData.([]api.Album)
		if len(albums) == 0 {
			return m, nil
		}
		return m.loadWithSpinner(loadAlbumsAllSongsCmd(m.api, albums))

	case ViewPlaylists:
		playlists, _ := m.viewData.([]api.Playlist)
		if idx < 0 || idx >= len(playlists) {
			return m, nil
		}
		pl := playlists[idx]
		return m.loadWithSpinner(loadPlaylistShuffleCmd(m.api, pl.ID))

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
		return m, m.showToast("Queue is empty", ToastInfo, toastShort)
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
	return m, m.showToast("Removed from queue", ToastInfo, toastShort)
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
		return m.loadWithSpinner(createPlaylistCmd(m.api, name))
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
		return m, m.showToast("No song to add", ToastError, toastShort)
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
	return m, m.showToast(fmt.Sprintf("Press d again to delete '%s'", p.Name), ToastInfo, toastShort)
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
		m.showToast("Removed from playlist", ToastSuccess, toastShort),
	)
}


// --- Quick actions popup ---

func (m *Model) updateQuickActions(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.quickActionsIdx > 0 {
			m.quickActionsIdx--
		}
	case "down", "j":
		if m.quickActionsIdx < len(discoverOptions)-1 {
			m.quickActionsIdx++
		}
	case "enter":
		m.showQuickActions = false
		return m.handleQuickAction(discoverOptions[m.quickActionsIdx])
	case "esc", "x":
		m.showQuickActions = false
	}
	return m, nil
}

func (m *Model) handleQuickAction(opt DiscoverOption) (tea.Model, tea.Cmd) {
	switch opt.Action {
	case "random":
		m.pushNav("Random Songs")
		return m.loadWithSpinner(loadRandomSongsCmd(m.api))
	case "similar":
		song := m.pl.CurrentSong()
		if song == nil {
			return m, m.showToast("No song currently playing", ToastError, toastLong)
		}
		m.pushNav("Similar")
		return m.loadWithSpinner(loadSimilarSongsCmd(m.api, song.ID, song.ArtistID))
	case "top":
		song := m.pl.CurrentSong()
		if song == nil {
			return m, m.showToast("No song currently playing", ToastError, toastLong)
		}
		m.pushNav("Top Songs")
		return m.loadWithSpinner(loadTopSongsCmd(m.api, song.Artist))
	}
	return m, nil
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
		m.showToast("Discovering Sonos speakers...", ToastInfo, toastShort),
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
	return m, m.showToast(fmt.Sprintf("Connected to %s", dev.Name), ToastSuccess, toastShort)
}

// disconnectSonos disables Sonos output and resumes the current song on local
// audio from the same position.
func (m *Model) disconnectSonos() (tea.Model, tea.Cmd) {
	song := m.pl.CurrentSong()
	if song == nil || song.ID == "" {
		m.pl.DisableSonos()
		return m, m.showToast("Switched to local audio", ToastInfo, toastShort)
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
