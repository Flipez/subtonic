package listenbrainz

// Recording represents a track from ListenBrainz.
type Recording struct {
	TrackName  string
	ArtistName string
	AlbumName  string
	MBID       string
	ArtistMBID string
}

// Release represents a fresh release from ListenBrainz.
type Release struct {
	Title      string
	ArtistName string
	Date       string
	MBID       string
}

// Playlist represents a ListenBrainz playlist (e.g. daily jams, weekly exploration).
type Playlist struct {
	MBID      string
	Title     string
	Creator   string
	Tracks    []Recording
	Algorithm string
}

// JSON envelope types for API responses

type sitewideRecordingsResponse struct {
	Payload struct {
		Recordings []struct {
			TrackName     string   `json:"track_name"`
			ArtistName    string   `json:"artist_name"`
			RecordingMBID string   `json:"recording_mbid"`
			ArtistMBIDs   []string `json:"artist_mbids"`
		} `json:"recordings"`
	} `json:"payload"`
}

type freshReleasesResponse struct {
	Payload struct {
		Releases []struct {
			ReleaseName string `json:"release_name"`
			ArtistName  string `json:"artist_credit_name"`
			ReleaseDate string `json:"release_date"`
			ReleaseMBID string `json:"release_mbid"`
		} `json:"releases"`
	} `json:"payload"`
}

type metadataLookupResponse struct {
	RecordingMBID string   `json:"recording_mbid"`
	ArtistMBIDs   []string `json:"artist_mbids"`
	RecordingName string   `json:"recording_name"`
	ArtistName    string   `json:"artist_credit_name"`
}

// popularRecording is a single entry in the top-recordings-for-artist response (plain JSON array).
type popularRecording struct {
	RecordingName string   `json:"recording_name"`
	ArtistName    string   `json:"artist_name"`
	RecordingMBID string   `json:"recording_mbid"`
	ArtistMBIDs   []string `json:"artist_mbids"`
}

// JSPF playlist format types

type jspfPlaylistsResponse struct {
	Playlists []struct {
		Playlist jspfPlaylist `json:"playlist"`
	} `json:"playlists"`
}

type jspfPlaylistResponse struct {
	Playlist jspfPlaylist `json:"playlist"`
}

type jspfPlaylist struct {
	Title      string `json:"title"`
	Creator    string `json:"creator"`
	Identifier string `json:"identifier"` // e.g. "https://listenbrainz.org/playlist/{mbid}"
	Annotation string `json:"annotation"`
	Extension  struct {
		MusicBrainz struct {
			AdditionalMetadata struct {
				AlgorithmMetadata struct {
					SourcePatch string `json:"source_patch"`
				} `json:"algorithm_metadata"`
			} `json:"additional_metadata"`
			CreatedFor string `json:"created_for"`
		} `json:"https://musicbrainz.org/doc/jspf#playlist"`
	} `json:"extension"`
	Track []jspfTrack `json:"track"`
}

type jspfTrack struct {
	Title      string   `json:"title"`
	Creator    string   `json:"creator"`
	Album      string   `json:"album"`
	Identifier []string `json:"identifier"` // ["https://musicbrainz.org/recording/{mbid}"]
}

type cfRecommendationsResponse struct {
	Payload struct {
		MBIDs []struct {
			RecordingMBID string `json:"recording_mbid"`
		} `json:"mbids"`
	} `json:"payload"`
}

// recordingMetadata is the per-MBID value in the /1/metadata/recording/ response map.
type recordingMetadata struct {
	Recording struct {
		Name string `json:"name"`
	} `json:"recording"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
}
