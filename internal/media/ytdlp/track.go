package ytdlp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
)

const youtubeWait = 3 * time.Second

type track struct {
	title      string
	duration   time.Duration
	webpageURL string
	streamData *trackStreamData

	mu sync.Mutex
	qs *querySource
}

type trackStreamData struct {
	validAfter  time.Time
	url         string
	httpHeaders map[string]string
}

var _ media.Track = (*track)(nil)

func (t *track) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

func (t *track) EstimatedDuration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.duration
}

func (t *track) WebpageURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.webpageURL
}

func (t *track) Stream(ctx context.Context) (io.ReadCloser, error) {
	if err := t.ensureStreamData(ctx); err != nil {
		return nil, err
	}

	// FUCK GOOGLE
	// the Youtube frontend is so shit nowadays that if you try to access the stream too fast the request fails cause a real client would be too slow to do it
	//
	// there should be a check here to ensure we are only waiting for Google bullshit to not fuck up Soundcloud playback, but I don't care
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Until(t.streamData.validAfter)):
	}

	return t.qs.httpAudio.Open(ctx, t.streamData.url, t.streamData.httpHeaders)
}

func (t *track) ensureStreamData(ctx context.Context) error {
	t.mu.Lock()
	if t.streamData != nil {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	parsedWebpageURL, err := url.ParseRequestURI(t.webpageURL)
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to parse webpage url: %w", err)
	}

	queryResult, err := t.qs.Query(ctx, parsedWebpageURL)
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to query for streamable track: %w", err)
	}

	if queryResult.Track == nil {
		return errors.New(errPrefix + "streamable track query did not return a track")
	}

	typedTrack, ok := queryResult.Track.(*track)
	if !ok {
		return errors.New(errPrefix + "streamable track query returned unknown track type")
	}

	if typedTrack.streamData == nil {
		return errors.New(errPrefix + "streamable track query returned track with no streaming data")
	}

	t.mu.Lock()
	if t.streamData == nil {
		t.title = typedTrack.title
		t.duration = typedTrack.duration
		t.webpageURL = typedTrack.webpageURL
		t.streamData = typedTrack.streamData
	}
	t.mu.Unlock()

	return nil
}

func (qs *querySource) extractTrack(yre *ytdlpResultEntry) (*track, error) {
	if yre.Title == "" {
		return nil, errors.New(errPrefix + "empty title")
	}

	t := &track{
		title:    yre.Title,
		duration: time.Duration(yre.Duration * float64(time.Second)),
		qs:       qs,
	}

	for _, format := range yre.Formats {
		if (format.Protocol == "http" || format.Protocol == "https") && format.URL != "" && format.ACodec != "none" && format.ACodec != "" {
			t.streamData = &trackStreamData{
				url:         format.URL,
				httpHeaders: format.HTTPHeaders,
				validAfter:  time.Now().Add(youtubeWait),
			}
			break
		}
	}

	if yre.WebpageURL == "" {
		t.webpageURL = yre.URL
	} else {
		t.webpageURL = yre.WebpageURL
	}

	return t, nil
}
