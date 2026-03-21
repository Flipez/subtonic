package api

import (
	"fmt"
	"net/url"
)

func (c *Client) Ping() error {
	_, err := c.get("ping", nil)
	return err
}

func (c *Client) GetArtists() ([]Artist, error) {
	body, err := c.get("getArtists", nil)
	if err != nil {
		return nil, err
	}
	if body.Artists == nil {
		return nil, nil
	}
	var artists []Artist
	for _, idx := range body.Artists.Index {
		artists = append(artists, idx.Artists...)
	}
	return artists, nil
}

func (c *Client) GetArtist(id string) (*Artist, error) {
	body, err := c.get("getArtist", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	return body.Artist, nil
}

func (c *Client) GetAlbum(id string) (*Album, error) {
	body, err := c.get("getAlbum", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	if body.Album == nil {
		return nil, nil
	}
	a := &Album{
		ID:        body.Album.ID,
		Name:      body.Album.Name,
		Artist:    body.Album.Artist,
		ArtistID:  body.Album.ArtistID,
		Year:      body.Album.Year,
		Genre:     body.Album.Genre,
		Starred:   body.Album.Starred,
		SongCount: body.Album.SongCount,
		Duration:  body.Album.Duration,
		Songs:     body.Album.Songs,
	}
	return a, nil
}

func (c *Client) Search(query string) (*SearchResult, error) {
	body, err := c.get("search3", url.Values{"query": {query}})
	if err != nil {
		return nil, err
	}
	if body.SearchResult3 == nil {
		return &SearchResult{}, nil
	}
	return &SearchResult{
		Artists: body.SearchResult3.Artists,
		Albums:  body.SearchResult3.Albums,
		Songs:   body.SearchResult3.Songs,
	}, nil
}

func (c *Client) GetPlaylists() ([]Playlist, error) {
	body, err := c.get("getPlaylists", nil)
	if err != nil {
		return nil, err
	}
	if body.Playlists == nil {
		return nil, nil
	}
	return body.Playlists.Playlists, nil
}

func (c *Client) GetPlaylist(id string) (*Playlist, error) {
	body, err := c.get("getPlaylist", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	return body.Playlist, nil
}

func (c *Client) GetAlbumList2(listType string, size, offset int) ([]Album, error) {
	body, err := c.get("getAlbumList2", url.Values{
		"type":   {listType},
		"size":   {fmt.Sprintf("%d", size)},
		"offset": {fmt.Sprintf("%d", offset)},
	})
	if err != nil {
		return nil, err
	}
	if body.AlbumList2 == nil {
		return nil, nil
	}
	return body.AlbumList2.Albums, nil
}

func (c *Client) GetPodcasts() ([]Podcast, error) {
	body, err := c.get("getPodcasts", url.Values{"includeEpisodes": {"true"}})
	if err != nil {
		return nil, err
	}
	if body.Podcasts == nil {
		return nil, nil
	}
	return body.Podcasts.Channels, nil
}

func (c *Client) GetPodcast(id string) (*Podcast, error) {
	body, err := c.get("getPodcasts", url.Values{"id": {id}, "includeEpisodes": {"true"}})
	if err != nil {
		return nil, err
	}
	if body.Podcasts == nil || len(body.Podcasts.Channels) == 0 {
		return nil, nil
	}
	return &body.Podcasts.Channels[0], nil
}

func (c *Client) Scrobble(id string) error {
	_, err := c.get("scrobble", url.Values{"id": {id}})
	return err
}

func (c *Client) GetRandomSongs(count int) ([]Song, error) {
	body, err := c.get("getRandomSongs", url.Values{
		"size": {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}
	if body.RandomSongs == nil {
		return nil, nil
	}
	return body.RandomSongs.Songs, nil
}

func (c *Client) GetStarred2() (*SearchResult, error) {
	body, err := c.get("getStarred2", nil)
	if err != nil {
		return nil, err
	}
	if body.Starred2 == nil {
		return &SearchResult{}, nil
	}
	return &SearchResult{
		Artists: body.Starred2.Artists,
		Albums:  body.Starred2.Albums,
		Songs:   body.Starred2.Songs,
	}, nil
}

func (c *Client) GetGenres() ([]Genre, error) {
	body, err := c.get("getGenres", nil)
	if err != nil {
		return nil, err
	}
	if body.Genres == nil {
		return nil, nil
	}
	return body.Genres.Genres, nil
}

func (c *Client) GetSongsByGenre(genre string, count, offset int) ([]Song, error) {
	body, err := c.get("getSongsByGenre", url.Values{
		"genre":  {genre},
		"count":  {fmt.Sprintf("%d", count)},
		"offset": {fmt.Sprintf("%d", offset)},
	})
	if err != nil {
		return nil, err
	}
	if body.SongsByGenre == nil {
		return nil, nil
	}
	return body.SongsByGenre.Songs, nil
}

func (c *Client) GetSimilarSongs(id string, count int) ([]Song, error) {
	body, err := c.get("getSimilarSongs", url.Values{
		"id":    {id},
		"count": {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}
	if body.SimilarSongs == nil {
		return nil, nil
	}
	return body.SimilarSongs.Songs, nil
}

func (c *Client) GetSimilarSongs2(id string, count int) ([]Song, error) {
	body, err := c.get("getSimilarSongs2", url.Values{
		"id":    {id},
		"count": {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}
	if body.SimilarSongs2 == nil {
		return nil, nil
	}
	return body.SimilarSongs2.Songs, nil
}

func (c *Client) GetTopSongs(artist string, count int) ([]Song, error) {
	body, err := c.get("getTopSongs", url.Values{
		"artist": {artist},
		"count":  {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}
	if body.TopSongs == nil {
		return nil, nil
	}
	return body.TopSongs.Songs, nil
}

func (c *Client) Star(id string) error {
	_, err := c.get("star", url.Values{"id": {id}})
	return err
}

func (c *Client) Unstar(id string) error {
	_, err := c.get("unstar", url.Values{"id": {id}})
	return err
}

func (c *Client) CreatePlaylist(name string) (*Playlist, error) {
	body, err := c.get("createPlaylist", url.Values{"name": {name}})
	if err != nil {
		return nil, err
	}
	return body.Playlist, nil
}

func (c *Client) UpdatePlaylist(id string, songIDsToAdd []string) error {
	params := url.Values{"playlistId": {id}}
	for _, sid := range songIDsToAdd {
		params.Add("songIdToAdd", sid)
	}
	_, err := c.get("updatePlaylist", params)
	return err
}

func (c *Client) DeletePlaylist(id string) error {
	_, err := c.get("deletePlaylist", url.Values{"id": {id}})
	return err
}

func (c *Client) SearchSongs(query string, count int) ([]Song, error) {
	body, err := c.get("search3", url.Values{
		"query":       {query},
		"songCount":   {fmt.Sprintf("%d", count)},
		"artistCount": {"0"},
		"albumCount":  {"0"},
	})
	if err != nil {
		return nil, err
	}
	if body.SearchResult3 == nil {
		return nil, nil
	}
	return body.SearchResult3.Songs, nil
}

func (c *Client) RemoveFromPlaylist(id string, indices []int) error {
	params := url.Values{"playlistId": {id}}
	for _, idx := range indices {
		params.Add("songIndexToRemove", fmt.Sprintf("%d", idx))
	}
	_, err := c.get("updatePlaylist", params)
	return err
}
