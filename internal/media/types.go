package media

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"
)

var ErrUnsupportedQuery = fmt.Errorf("media: unsupported query")

type Track interface {
	GetTitle() string
	GetDuration() time.Duration
	GetURL() string
	GetStream(ctx context.Context) (io.ReadCloser, error)
}

type Playlist interface {
	GetTitle() string
	GetURL() string
	GetTracks() []Track
}

type QueryPreference int

const (
	QueryPreferenceTrack QueryPreference = iota
	QueryPreferencePlaylist
)

type SearchQueryOptions struct {
	MaxResults int
}

type URLQueryOptions struct {
	Preference QueryPreference
}

type URLQueryResult struct {
	Track    Track
	Playlist Playlist
}

type TrackProvider interface {
	QueryBySearch(ctx context.Context, query string, options SearchQueryOptions) ([]Track, error)
	QueryByURL(ctx context.Context, query *url.URL, options URLQueryOptions) (URLQueryResult, error)
}
