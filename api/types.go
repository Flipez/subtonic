package api

type Song struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Duration   int    `json:"duration"`
	Track      int    `json:"track"`
	Suffix     string `json:"suffix"`
	Genre      string `json:"genre"`
	BitRate    int    `json:"bitRate"`
	Starred    string `json:"starred"`
	DiscNumber int    `json:"discNumber"`
	Size       int64  `json:"size"`
	ArtistID   string `json:"artistId"`
}

type Album struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	ArtistID  string `json:"artistId"`
	Year      int    `json:"year"`
	Genre     string `json:"genre"`
	Starred   string `json:"starred"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Songs     []Song `json:"song"`
}

type Artist struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AlbumCount int     `json:"albumCount"`
	Starred    string  `json:"starred"`
	Albums     []Album `json:"album"`
}

type Playlist struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Songs     []Song `json:"entry"`
}

type Podcast struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Status   string           `json:"status"`
	Episodes []PodcastEpisode `json:"episode"`
}

type PodcastEpisode struct {
	ID       string `json:"id"`
	StreamID string `json:"streamId"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Duration int    `json:"duration"`
	Suffix   string `json:"suffix"`
	BitRate  int    `json:"bitRate"`
	Size     int64  `json:"size"`
	Year     int    `json:"year"`
}

type Genre struct {
	Name       string `json:"value"`
	SongCount  int    `json:"songCount"`
	AlbumCount int    `json:"albumCount"`
}

type SearchResult struct {
	Artists []Artist `json:"artist"`
	Albums  []Album  `json:"album"`
	Songs   []Song   `json:"song"`
}

// JSON envelope types

type subsonicResponse struct {
	SubsonicResponse subsonicResponseBody `json:"subsonic-response"`
}

type subsonicResponseBody struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	ServerVersion string            `json:"serverVersion"`
	Type          string            `json:"type"`
	Error         *subsonicError    `json:"error"`
	Artists       *artistsResult    `json:"artists"`
	Artist        *Artist           `json:"artist"`
	Album         *albumResult      `json:"album"`
	SearchResult3 *searchResult3    `json:"searchResult3"`
	AlbumList2    *albumList2Result `json:"albumList2"`
	Playlists     *playlistsResult  `json:"playlists"`
	Playlist      *Playlist         `json:"playlist"`
	Podcasts      *podcastsResult   `json:"podcasts"`
	Genres        *genresResult        `json:"genres"`
	RandomSongs   *randomSongsResult   `json:"randomSongs"`
	Starred2      *starred2Result      `json:"starred2"`
	SimilarSongs  *similarSongsResult   `json:"similarSongs"`
	SimilarSongs2 *similarSongs2Result `json:"similarSongs2"`
	TopSongs      *topSongsResult      `json:"topSongs"`
	SongsByGenre  *songsByGenreResult  `json:"songsByGenre"`
}

type subsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type artistsResult struct {
	Index []artistIndex `json:"index"`
}

type artistIndex struct {
	Name    string   `json:"name"`
	Artists []Artist `json:"artist"`
}

type albumResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	ArtistID  string `json:"artistId"`
	Year      int    `json:"year"`
	Genre     string `json:"genre"`
	Starred   string `json:"starred"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Songs     []Song `json:"song"`
}

type searchResult3 struct {
	Artists []Artist `json:"artist"`
	Albums  []Album  `json:"album"`
	Songs   []Song   `json:"song"`
}

type albumList2Result struct {
	Albums []Album `json:"album"`
}

type playlistsResult struct {
	Playlists []Playlist `json:"playlist"`
}

type podcastsResult struct {
	Channels []Podcast `json:"channel"`
}

type genresResult struct {
	Genres []Genre `json:"genre"`
}

type randomSongsResult struct {
	Songs []Song `json:"song"`
}

type starred2Result struct {
	Artists []Artist `json:"artist"`
	Albums  []Album  `json:"album"`
	Songs   []Song   `json:"song"`
}

type similarSongsResult struct {
	Songs []Song `json:"song"`
}

type similarSongs2Result struct {
	Songs []Song `json:"song"`
}

type topSongsResult struct {
	Songs []Song `json:"song"`
}

type songsByGenreResult struct {
	Songs []Song `json:"song"`
}
