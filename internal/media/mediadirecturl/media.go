package mediadirecturl

import (
	"context"
	"io"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/rangeread"
)

type DirectURLTrack struct {
	Title string
	URL   string
}

func (t *DirectURLTrack) GetTitle() string {
	return t.Title
}

func (t *DirectURLTrack) GetDuration() time.Duration {
	return 0
}

func (t *DirectURLTrack) GetURL() string {
	return t.URL
}

func (t *DirectURLTrack) GetArtworkURL() string {
	return ""
}

func (t *DirectURLTrack) GetStream(ctx context.Context) (io.ReadCloser, error) {
	return rangeread.NewRangeReader(ctx, rangeread.RangeReaderOptions{
		URL: t.URL,
	})
}

var _ media.Track = (*DirectURLTrack)(nil)
