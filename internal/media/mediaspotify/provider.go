package mediaspotify

import (
	"context"
	"fmt"
	"net/url"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/spotifyinfo"
)

type provider struct {
	infoClient              *spotifyinfo.Client
	streamableTrackResolver media.TrackProvider
}

func NewProvider(infoClient *spotifyinfo.Client, streamableTrackResolver media.TrackProvider) *provider {
	return &provider{
		infoClient:              infoClient,
		streamableTrackResolver: streamableTrackResolver,
	}
}

func (p *provider) QueryBySearch(ctx context.Context, query string, options media.SearchQueryOptions) ([]media.Track, error) {
	return nil, media.ErrUnsupportedQuery
}

func (p *provider) QueryByURL(ctx context.Context, query *url.URL, options media.URLQueryOptions) (media.URLQueryResult, error) {
	uri, err := spotifyinfo.URLToURI(query)
	if err != nil {
		return media.URLQueryResult{}, media.ErrUnsupportedQuery
	}

	switch uri.Type {
	case spotifyinfo.EntityTypeTrack:
		trackInfo, err := p.infoClient.GetTrack(ctx, uri)
		if err != nil {
			return media.URLQueryResult{}, fmt.Errorf(errPrefix+"failed to get track info: %w", err)
		}

		return media.URLQueryResult{
			Track: &track{
				trackInfo: trackInfo,
				provider:  p,
			},
		}, nil
	case spotifyinfo.EntityTypeAlbum, spotifyinfo.EntityTypePlaylist:
		trackListInfo, err := p.infoClient.GetTrackList(ctx, uri)
		if err != nil {
			return media.URLQueryResult{}, fmt.Errorf(errPrefix+"failed to get track list info: %w", err)
		}

		return media.URLQueryResult{
			Playlist: &playlist{
				trackListInfo: trackListInfo,
				provider:      p,
			},
		}, nil
	default:
		return media.URLQueryResult{}, media.ErrUnsupportedQuery
	}
}
