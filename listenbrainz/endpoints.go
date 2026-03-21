package listenbrainz

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) GetTrendingRecordings(count int) ([]Recording, error) {
	data, err := c.get("/1/stats/sitewide/recordings", url.Values{
		"range": {"week"},
		"count": {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}

	var resp sitewideRecordingsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var recordings []Recording
	for _, r := range resp.Payload.Recordings {
		rec := Recording{
			TrackName:  r.TrackName,
			ArtistName: r.ArtistName,
			MBID:       r.RecordingMBID,
		}
		if len(r.ArtistMBIDs) > 0 {
			rec.ArtistMBID = r.ArtistMBIDs[0]
		}
		recordings = append(recordings, rec)
	}
	return recordings, nil
}

func (c *Client) GetFreshReleases() ([]Release, error) {
	data, err := c.get("/1/explore/fresh-releases/", url.Values{})
	if err != nil {
		return nil, err
	}

	var resp freshReleasesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var releases []Release
	for i, r := range resp.Payload.Releases {
		if i >= 10 {
			break
		}
		releases = append(releases, Release{
			Title:      r.ReleaseName,
			ArtistName: r.ArtistName,
			Date:       r.ReleaseDate,
			MBID:       r.ReleaseMBID,
		})
	}
	return releases, nil
}

func (c *Client) GetPopularByArtist(artistName, recordingName string) ([]Recording, error) {
	// Step 1: metadata lookup for artist MBID
	data, err := c.get("/1/metadata/lookup/", url.Values{
		"artist_name":    {artistName},
		"recording_name": {recordingName},
	})
	if err != nil {
		return nil, err
	}

	var meta metadataLookupResponse
	if err := json.Unmarshal(data, &meta); err != nil || len(meta.ArtistMBIDs) == 0 {
		return nil, nil
	}

	artistMBID := meta.ArtistMBIDs[0]

	// Step 2: popular recordings for the artist (response is a plain JSON array)
	data, err = c.get(fmt.Sprintf("/1/popularity/top-recordings-for-artist/%s", artistMBID), url.Values{})
	if err != nil {
		return nil, err
	}

	var popResp []popularRecording
	if err := json.Unmarshal(data, &popResp); err != nil {
		return nil, err
	}

	var recordings []Recording
	for _, r := range popResp {
		recordings = append(recordings, Recording{
			TrackName:  r.RecordingName,
			ArtistName: r.ArtistName,
			MBID:       r.RecordingMBID,
		})
	}
	return recordings, nil
}

func extractMBID(identifier string) string {
	if idx := strings.LastIndex(identifier, "/"); idx >= 0 {
		return identifier[idx+1:]
	}
	return ""
}

func parseJSPFTrack(t jspfTrack) Recording {
	rec := Recording{
		TrackName:  t.Title,
		ArtistName: t.Creator,
		AlbumName:  t.Album,
	}
	if len(t.Identifier) > 0 {
		rec.MBID = extractMBID(t.Identifier[0])
	}
	return rec
}

func parseJSPFPlaylist(p jspfPlaylist) Playlist {
	pl := Playlist{
		Title:     p.Title,
		Creator:   p.Creator,
		MBID:      extractMBID(p.Identifier),
		Algorithm: p.Extension.MusicBrainz.AdditionalMetadata.AlgorithmMetadata.SourcePatch,
	}
	for _, t := range p.Track {
		pl.Tracks = append(pl.Tracks, parseJSPFTrack(t))
	}
	return pl
}

func (c *Client) GetUserPlaylists(username string) ([]Playlist, error) {
	data, err := c.get(fmt.Sprintf("/1/user/%s/playlists", url.PathEscape(username)), url.Values{})
	if err != nil {
		return nil, err
	}

	var resp jspfPlaylistsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var playlists []Playlist
	for _, p := range resp.Playlists {
		playlists = append(playlists, parseJSPFPlaylist(p.Playlist))
	}
	return playlists, nil
}

func (c *Client) GetPlaylistsCreatedFor(username string) ([]Playlist, error) {
	data, err := c.get(fmt.Sprintf("/1/user/%s/playlists/createdfor", url.PathEscape(username)), url.Values{})
	if err != nil {
		return nil, err
	}

	var resp jspfPlaylistsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var playlists []Playlist
	for _, p := range resp.Playlists {
		playlists = append(playlists, parseJSPFPlaylist(p.Playlist))
	}
	return playlists, nil
}

func (c *Client) GetPlaylist(playlistMBID string) (*Playlist, error) {
	data, err := c.get(fmt.Sprintf("/1/playlist/%s", url.PathEscape(playlistMBID)), url.Values{})
	if err != nil {
		return nil, err
	}

	var resp jspfPlaylistResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	pl := parseJSPFPlaylist(resp.Playlist)
	return &pl, nil
}

func (c *Client) GetRecommendations(username string, count int) ([]Recording, error) {
	data, err := c.get(fmt.Sprintf("/1/cf/recommendation/user/%s/recording", url.PathEscape(username)), url.Values{
		"count": {fmt.Sprintf("%d", count)},
	})
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var resp cfRecommendationsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	if len(resp.Payload.MBIDs) == 0 {
		return nil, nil
	}

	// Batch resolve MBIDs to metadata
	var mbids []string
	for _, m := range resp.Payload.MBIDs {
		mbids = append(mbids, m.RecordingMBID)
	}

	return c.resolveRecordingMBIDs(mbids)
}

func (c *Client) resolveRecordingMBIDs(mbids []string) ([]Recording, error) {
	data, err := c.get("/1/metadata/recording/", url.Values{
		"recording_mbids": {strings.Join(mbids, ",")},
		"inc":             {"artist"},
	})
	if err != nil {
		return nil, err
	}

	var resp map[string]recordingMetadata
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	// Preserve the original order from the input MBIDs
	var recordings []Recording
	for _, mbid := range mbids {
		meta, ok := resp[mbid]
		if !ok || meta.Recording.Name == "" {
			continue
		}
		recordings = append(recordings, Recording{
			TrackName:  meta.Recording.Name,
			ArtistName: meta.Artist.Name,
			MBID:       mbid,
		})
	}
	return recordings, nil
}
