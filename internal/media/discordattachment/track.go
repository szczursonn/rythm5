package discordattachment

import (
	"context"
	"io"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
)

type track struct {
	title    string
	url      string
	duration time.Duration

	s *Source
}

var _ media.Track = (*track)(nil)

func (t *track) Title() string {
	return t.title
}

func (t *track) EstimatedDuration() time.Duration {
	return t.duration
}

func (t *track) WebpageURL() string {
	return t.url
}

func (t *track) Stream(ctx context.Context) (io.ReadCloser, error) {
	return t.s.httpAudio.Open(ctx, t.url, nil)
}
