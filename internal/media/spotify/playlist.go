package spotify

import (
	"context"
	"strings"
	"time"

	"github.com/szczursonn/rythm5/internal/media"
)

type playlist struct {
	title string
	url   string

	tracks []media.Track
}

var _ media.Playlist = (*playlist)(nil)

func (p *playlist) Title() string {
	return p.title
}

func (p *playlist) WebpageURL() string {
	return p.url
}

func (p *playlist) Tracks() []media.Track {
	return p.tracks
}

func (qs *querySource) getPlaylist(ctx context.Context, u uri) (*playlist, error) {
	data, err := qs.getNextJSData(ctx, u)
	if err != nil {
		return nil, err
	}

	tracks := make([]media.Track, 0, len(data.Props.PageProps.State.Data.Entity.TrackList))
	for _, partialTrack := range data.Props.PageProps.State.Data.Entity.TrackList {
		artistsNames := strings.Split(partialTrack.Subtitle, ",")

		authorForQuery := ""
		for _, artistName := range artistsNames {
			artistName = strings.TrimSpace(artistName)
			if artistName != "" {
				authorForQuery = artistName
				break
			}
		}

		tracks = append(tracks, &track{
			title:          partialTrack.Title,
			duration:       time.Duration(partialTrack.Duration) * time.Millisecond,
			url:            partialTrack.URI.URL().String(),
			authorForQuery: authorForQuery,
			qs:             qs,
		})
	}

	playlistURI := uri{
		Type: data.Props.PageProps.State.Data.Entity.Type,
		ID:   data.Props.PageProps.State.Data.Entity.Id,
	}

	return &playlist{
		title:  data.Props.PageProps.State.Data.Entity.Title,
		url:    playlistURI.URL().String(),
		tracks: tracks,
	}, nil
}
