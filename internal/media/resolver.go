package media

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var ErrUnsupportedQuery = errors.New("media: unsupported query")

const QuerySchemeGenericSearch = "search"

type QueryResult struct {
	Track    Track
	Playlist Playlist
}

type QuerySource interface {
	SupportedPrefixSchemes() []string
	Query(ctx context.Context, query *url.URL) (QueryResult, error)
}

type QueryResolver struct {
	handlers               []QuerySource
	supportedPrefixSchemes map[string]struct{}
}

func NewQueryResolver(handlers ...QuerySource) *QueryResolver {
	supportedPrefixSchemes := map[string]struct{}{}
	for _, handler := range handlers {
		for _, scheme := range handler.SupportedPrefixSchemes() {
			supportedPrefixSchemes[scheme] = struct{}{}
		}
	}

	return &QueryResolver{
		handlers:               handlers,
		supportedPrefixSchemes: supportedPrefixSchemes,
	}
}

func (qr *QueryResolver) Query(ctx context.Context, queryStr string) (QueryResult, error) {
	query := qr.parseQueryString(queryStr)

	for _, handler := range qr.handlers {
		qr, err := handler.Query(ctx, query)
		if err != nil && errors.Is(err, ErrUnsupportedQuery) {
			continue
		}

		return qr, err
	}

	return QueryResult{}, ErrUnsupportedQuery
}

func (qr *QueryResolver) parseQueryString(queryStr string) *url.URL {
	queryStr = strings.TrimSpace(queryStr)

	u, err := url.ParseRequestURI(queryStr)
	if err == nil {
		return u
	}

	potentialScheme, potentialQuery, _ := strings.Cut(queryStr, ":")
	if _, ok := qr.supportedPrefixSchemes[potentialScheme]; ok {
		return &url.URL{
			Scheme: potentialScheme,
			Path:   strings.TrimSpace(potentialQuery),
		}
	}

	return &url.URL{
		Scheme: QuerySchemeGenericSearch,
		Path:   queryStr,
	}
}
