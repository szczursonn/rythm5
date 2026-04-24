package spotify

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
)

type track struct {
	title          string
	duration       time.Duration
	url            string
	authorForQuery string

	qs            *querySource
	mu            sync.Mutex
	resolvedTrack media.Track
}

var _ media.Track = (*track)(nil)

func (t *track) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.Title()
	}

	return t.title
}

func (t *track) EstimatedDuration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.EstimatedDuration()
	}

	return t.duration
}

func (t *track) WebpageURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack != nil {
		return t.resolvedTrack.WebpageURL()
	}

	return t.url
}

func (t *track) Stream(ctx context.Context) (io.ReadCloser, error) {
	if err := t.ensureTrackIsResolved(ctx); err != nil {
		return nil, err
	}

	return t.resolvedTrack.Stream(ctx)
}

func (t *track) ensureTrackIsResolved(ctx context.Context) error {
	t.mu.Lock()
	if t.resolvedTrack != nil {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()

	query := ""
	if len(t.authorForQuery) > 0 {
		query = t.authorForQuery + " "
	}
	query += t.title

	result, err := t.qs.streamableQuerySource.Query(ctx, &url.URL{
		Scheme: media.QuerySchemeGenericSearch,
		Path:   query,
	})
	if err != nil {
		return fmt.Errorf(errPrefix+"failed to search using another provider: %w", err)
	}

	if result.Track == nil {
		return fmt.Errorf(errPrefix + "search using another provider returned a playlist")
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolvedTrack == nil {
		t.resolvedTrack = result.Track
	}

	return nil
}

func (qs *querySource) getTrack(ctx context.Context, u uri) (*track, error) {
	data, err := qs.getNextJSData(ctx, u)
	if err != nil {
		return nil, err
	}

	authorForQuery := ""
	if len(data.Props.PageProps.State.Data.Entity.Artists) > 0 {
		authorForQuery = strings.TrimSpace(data.Props.PageProps.State.Data.Entity.Artists[0].Name)
	}

	return &track{
		title:          data.Props.PageProps.State.Data.Entity.Title,
		duration:       time.Duration(data.Props.PageProps.State.Data.Entity.Duration) * time.Millisecond,
		url:            u.URL().String(),
		authorForQuery: authorForQuery,
		qs:             qs,
	}, nil
}
