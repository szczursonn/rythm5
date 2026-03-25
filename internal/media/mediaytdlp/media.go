package mediaytdlp

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/rangeread"
	"github.com/szczursonn/rythm5/internal/ytdlp"
)

const errPrefix = "media/ytdlp: "

type track struct {
	audioResourceInfo *ytdlp.AudioResourceInfo

	mu       sync.Mutex
	provider *provider
}

func (t *track) GetTitle() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.audioResourceInfo.Title
}

func (t *track) GetDuration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.audioResourceInfo.Duration
}

func (t *track) GetURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.audioResourceInfo.WebpageURL
}

func (t *track) GetStream(ctx context.Context) (io.ReadCloser, error) {
	if err := t.ensureStreamData(ctx); err != nil {
		return nil, err
	}

	return rangeread.NewRangeReader(ctx, rangeread.RangeReaderOptions{
		URL:         t.audioResourceInfo.StreamURL,
		HTTPHeaders: t.audioResourceInfo.StreamHTTPHeaders,
	})
}

func (t *track) ensureStreamData(ctx context.Context) error {
	t.mu.Lock()
	if t.audioResourceInfo.StreamURL != "" {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	parsedWebpageURL, err := url.ParseRequestURI(t.audioResourceInfo.WebpageURL)
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to parse webpage url: %w", err)
	}

	queryResult, err := t.provider.QueryByURL(ctx, parsedWebpageURL, media.URLQueryOptions{
		Preference: media.QueryPreferenceTrack,
	})
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to query for streamable track: %w", err)
	}

	if queryResult.Track == nil {
		return fmt.Errorf(errPrefix + "streamable track query did not return a track")
	}

	typedTrack, ok := queryResult.Track.(*track)
	if !ok {
		return fmt.Errorf(errPrefix + "streamable track query returned unknown track type")
	}

	if typedTrack.audioResourceInfo.StreamURL == "" {
		return fmt.Errorf(errPrefix + "streamable track query returned track with no streaming data")
	}

	t.mu.Lock()
	if t.audioResourceInfo.StreamURL == "" {
		t.audioResourceInfo = typedTrack.audioResourceInfo
	}
	t.mu.Unlock()

	return nil
}

type playlist struct {
	playlistInfo *ytdlp.AudioResourcePlaylistInfo
	provider     *provider
}

func (p *playlist) GetTitle() string {
	return p.playlistInfo.Title
}

func (p *playlist) GetURL() string {
	return p.playlistInfo.WebpageURL
}

func (p *playlist) GetTracks() []media.Track {
	genericTracks := make([]media.Track, 0, len(p.playlistInfo.Entries))
	for _, audioResourceInfo := range p.playlistInfo.Entries {
		genericTracks = append(genericTracks, &track{
			audioResourceInfo: audioResourceInfo,
			provider:          p.provider,
		})
	}
	return genericTracks
}

var _ media.Track = (*track)(nil)
var _ media.Playlist = (*playlist)(nil)
