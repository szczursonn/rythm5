package media

import (
	"context"
	"io"
	"time"
)

type Track interface {
	Title() string
	EstimatedDuration() time.Duration
	WebpageURL() string
	Stream(ctx context.Context) (io.ReadCloser, error)
}

type Playlist interface {
	Title() string
	WebpageURL() string
	Tracks() []Track
}
