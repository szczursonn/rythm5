package mediaytdlp

import (
	"context"
	"net/url"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/ytdlp"
)

type provider struct {
	ytdlpClient *ytdlp.Client
}

func NewProvider(ytdlpClient *ytdlp.Client) media.TrackProvider {
	return &provider{
		ytdlpClient: ytdlpClient,
	}
}

func (p *provider) QueryBySearch(ctx context.Context, query string, options media.SearchQueryOptions) ([]media.Track, error) {
	audioResourcesInfos, err := p.ytdlpClient.GetAudioResourcesByYoutubeSearch(ctx, query, options.MaxResults)
	if err != nil {
		return nil, err
	}

	tracks := make([]media.Track, 0, len(audioResourcesInfos))
	for _, audioResourceInfo := range audioResourcesInfos {
		tracks = append(tracks, &track{
			audioResourceInfo: audioResourceInfo,
			provider:          p,
		})
	}

	return tracks, nil
}

func (p *provider) QueryByURL(ctx context.Context, query *url.URL, options media.URLQueryOptions) (media.URLQueryResult, error) {
	switch query.Host {
	case "www.youtube.com", "youtube.com", "music.youtube.com", "www.youtu.be", "youtu.be", "soundcloud.com", "on.soundcloud.com":
		break
	default:
		return media.URLQueryResult{}, media.ErrUnsupportedQuery
	}

	preferPlaylist := false
	if options.Preference == media.QueryPreferencePlaylist {
		preferPlaylist = true
	}

	queryResult, err := p.ytdlpClient.GetAudioResourcesByURL(ctx, query.String(), preferPlaylist)
	if err != nil {
		return media.URLQueryResult{}, err
	}

	if queryResult.SingleInfo != nil {
		return media.URLQueryResult{
			Track: &track{
				audioResourceInfo: queryResult.SingleInfo,
				provider:          p,
			},
		}, nil
	}

	return media.URLQueryResult{
		Playlist: &playlist{
			playlistInfo: queryResult.PlaylistInfo,
			provider:     p,
		},
	}, nil
}
