package mediaspotify

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/spotifyinfo"
)

const errPrefix = "media/spotify: "

type track struct {
	trackInfo *spotifyinfo.Track
	provider  *provider

	mu            sync.Mutex
	resolvedTrack media.Track
}

func (t *track) GetTitle() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.GetTitle()
	}

	return t.trackInfo.Title
}

func (t *track) GetDuration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.GetDuration()
	}

	return t.trackInfo.Duration
}

func (t *track) GetURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.GetURL()
	}

	return t.trackInfo.URI.URL().String()
}

func (t *track) GetStream(ctx context.Context) (io.ReadCloser, error) {
	if err := t.ensureTrackIsResolved(ctx); err != nil {
		return nil, err
	}

	return t.resolvedTrack.GetStream(ctx)
}

func (t *track) ensureTrackIsResolved(ctx context.Context) error {
	t.mu.Lock()
	if t.resolvedTrack != nil {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	query := ""
	if len(t.trackInfo.Authors) > 0 {
		query = t.trackInfo.Authors[0].Name + " "
	}
	query += t.trackInfo.Title

	streamableTracks, err := t.provider.streamableTrackResolver.QueryBySearch(ctx, query, media.SearchQueryOptions{
		MaxResults: 1,
	})
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to search using another provider: %w", err)
	}

	if len(streamableTracks) == 0 {
		return fmt.Errorf(errPrefix + "search using another provider returned 0 results")
	}

	t.mu.Lock()
	if t.resolvedTrack == nil {
		t.resolvedTrack = streamableTracks[0]
	}
	t.mu.Unlock()

	return nil
}

type playlist struct {
	trackListInfo *spotifyinfo.TrackList
	provider      *provider
}

func (p *playlist) GetTitle() string {
	return p.trackListInfo.Title
}

func (p *playlist) GetURL() string {
	return p.trackListInfo.URI.URL().String()
}

func (p *playlist) GetTracks() []media.Track {
	tracks := make([]media.Track, 0, len(p.trackListInfo.Tracks))

	for _, trackInfo := range p.trackListInfo.Tracks {
		tracks = append(tracks, &track{
			trackInfo: trackInfo,
			provider:  p.provider,
		})
	}

	return tracks
}

var _ media.Track = (*track)(nil)
var _ media.Playlist = (*playlist)(nil)
