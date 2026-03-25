package media

import (
	"context"
	"errors"
	"net/url"
)

func NewMultiProvider(providers []TrackProvider) TrackProvider {
	return &multiProvider{
		providers: providers,
	}
}

type multiProvider struct {
	providers []TrackProvider
}

func (mp *multiProvider) QueryBySearch(ctx context.Context, query string, options SearchQueryOptions) ([]Track, error) {
	for _, provider := range mp.providers {
		tracks, err := provider.QueryBySearch(ctx, query, options)
		if err != nil && errors.Is(err, ErrUnsupportedQuery) {
			continue
		}

		return tracks, err
	}

	return nil, ErrUnsupportedQuery
}

func (mp *multiProvider) QueryByURL(ctx context.Context, query *url.URL, options URLQueryOptions) (URLQueryResult, error) {
	for _, provider := range mp.providers {
		result, err := provider.QueryByURL(ctx, query, options)
		if err != nil && errors.Is(err, ErrUnsupportedQuery) {
			continue
		}

		return result, err
	}

	return URLQueryResult{}, ErrUnsupportedQuery
}
