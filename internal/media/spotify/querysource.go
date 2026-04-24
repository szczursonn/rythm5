package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/szczursonn/rythm5/internal/media"
)

const errPrefix = "media/spotify: "

type querySource struct {
	httpClient            *http.Client
	streamableQuerySource media.QuerySource
}

type QueryHandlerOptions struct {
	HttpClient            *http.Client
	StreamableQuerySource media.QuerySource
}

var _ media.QuerySource = (*querySource)(nil)

func NewQueryHandler(opts QueryHandlerOptions) media.QuerySource {
	if opts.StreamableQuerySource == nil {
		panic(errPrefix + "missing streamable query source")
	}

	return &querySource{
		httpClient:            opts.HttpClient,
		streamableQuerySource: opts.StreamableQuerySource,
	}
}

func (qs *querySource) SupportedPrefixSchemes() []string {
	return nil
}

func (qs *querySource) Query(ctx context.Context, query *url.URL) (media.QueryResult, error) {
	switch query.Scheme {
	case "http", "https":
		u, err := urlToURI(query)
		if err != nil {
			return media.QueryResult{}, media.ErrUnsupportedQuery
		}

		return qs.queryURI(ctx, u)
	default:
		return media.QueryResult{}, media.ErrUnsupportedQuery
	}
}

func (qs *querySource) queryURI(ctx context.Context, u uri) (media.QueryResult, error) {
	switch u.Type {
	case entityTypeTrack:
		t, err := qs.getTrack(ctx, u)
		if err != nil {
			return media.QueryResult{}, fmt.Errorf(errPrefix+"failed to get track info: %w", err)
		}

		return media.QueryResult{
			Track: t,
		}, nil
	case entityTypeAlbum, entityTypePlaylist:
		p, err := qs.getPlaylist(ctx, u)
		if err != nil {
			return media.QueryResult{}, fmt.Errorf(errPrefix+"failed to get track list info: %w", err)
		}

		return media.QueryResult{
			Playlist: p,
		}, nil
	default:
		return media.QueryResult{}, media.ErrUnsupportedQuery
	}
}

type nextJSData struct {
	Props struct {
		PageProps struct {
			Status int `json:"status"`
			State  struct {
				Data struct {
					Entity struct {
						Type     entityType `json:"type"`
						Id       string     `json:"id"`
						Title    string     `json:"title"`
						Subtitle string     `json:"subtitle"`
						Duration int        `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
							URI  uri    `json:"uri"`
						} `json:"artists"`
						TrackList []struct {
							URI      uri    `json:"uri"`
							Title    string `json:"title"`
							Subtitle string `json:"subtitle"`
							Duration int    `json:"duration"`
						} `json:"trackList"`
					} `json:"entity"`
				} `json:"data"`
			} `json:"state"`
		} `json:"pageProps"`
	} `json:"props"`
}

type entityType string

const (
	entityTypeTrack    entityType = "track"
	entityTypePlaylist entityType = "playlist"
	entityTypeAlbum    entityType = "album"
	entityTypeArtist   entityType = "artist"
)

func (et entityType) IsValid() bool {
	switch et {
	case entityTypeTrack, entityTypePlaylist, entityTypeAlbum, entityTypeArtist:
		return true
	}
	return false
}

type uri struct {
	ID   string
	Type entityType
}

func (u uri) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *uri) UnmarshalJSON(data []byte) error {
	str := ""
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	uriVal, err := parseURI(str)
	if err != nil {
		return err
	}
	*u = uriVal

	return nil
}

func (u *uri) String() string {
	return fmt.Sprintf("spotify:%s:%s", u.Type, u.ID)
}

func (u *uri) URL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "open.spotify.com",
		Path:   fmt.Sprintf("/%s/%s", u.Type, u.ID),
	}
}

func parseURI(rawURI string) (uri, error) {
	uriParts := strings.Split(rawURI, ":")
	if len(uriParts) != 3 {
		return uri{}, fmt.Errorf(errPrefix+"uri has invalid amount of parts: %s", rawURI)
	}
	if uriParts[0] != "spotify" {
		return uri{}, fmt.Errorf(errPrefix+"uri does not start with \"spotify\": %s", rawURI)
	}

	et := entityType(uriParts[1])
	if !et.IsValid() {
		return uri{}, fmt.Errorf(errPrefix+"uri has invalid entity type: %s", rawURI)
	}

	return uri{
		ID:   uriParts[2],
		Type: et,
	}, nil
}

func urlToURI(parsedURL *url.URL) (uri, error) {
	if parsedURL.Host != "open.spotify.com" {
		return uri{}, fmt.Errorf(errPrefix+"invalid spotify url host: %s", parsedURL.Host)
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 2 {
		return uri{}, fmt.Errorf(errPrefix+"invalid spotify url path parts amount: %s", parsedURL.Path)
	}

	et := entityType(pathParts[0])
	if !et.IsValid() {
		return uri{}, fmt.Errorf(errPrefix+"invalid spotify url entity type: %s", pathParts[0])
	}

	return uri{
		ID:   pathParts[1],
		Type: et,
	}, nil
}

func (qs *querySource) getNextJSData(ctx context.Context, u uri) (*nextJSData, error) {
	httpClient := qs.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	endpoint := url.URL{
		Scheme: "https",
		Host:   "open.spotify.com",
		Path:   fmt.Sprintf("/embed/%s/%s", u.Type, u.ID),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to create request: %w", err)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to do request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errPrefix+"unexpected status code: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to read response body: %w", err)
	}

	nextDataStartIndex := bytes.LastIndex(body, []byte(`{"props":{"pageProps":`))
	if nextDataStartIndex == -1 {
		return nil, fmt.Errorf(errPrefix + "failed to find nextjs data section start")
	}

	nextDataEndIndex := bytes.LastIndex(body[nextDataStartIndex:], []byte("</script>"))
	if nextDataEndIndex == -1 {
		return nil, fmt.Errorf(errPrefix + "failed to find nextjs data section end")
	}

	data := &nextJSData{}
	if err = json.Unmarshal(body[nextDataStartIndex:nextDataStartIndex+nextDataEndIndex], data); err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to unmarshal nextjs data: %w", err)
	}

	if data.Props.PageProps.Status != 0 {
		return nil, fmt.Errorf(errPrefix+"unexpected status code in nextjs data: %d", data.Props.PageProps.Status)
	}

	return data, nil
}
