package spotifyinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const errPrefix = "spotifyinfo: "

type Client struct {
	HTTPClient *http.Client
}

type Track struct {
	URI      URI
	Title    string
	Authors  []Author
	Duration time.Duration
}

type TrackList struct {
	URI     URI
	Title   string
	Authors []Author
	Tracks  []*Track
}

type Author struct {
	URI  URI
	Name string
}

type nextJSData struct {
	Props struct {
		PageProps struct {
			Status int `json:"status"`
			State  struct {
				Data struct {
					Entity struct {
						Type     EntityType `json:"type"`
						Id       string     `json:"id"`
						Title    string     `json:"title"`
						Subtitle string     `json:"subtitle"`
						Duration int        `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
							URI  URI    `json:"uri"`
						} `json:"artists"`
						TrackList []struct {
							URI      URI    `json:"uri"`
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

func (c *Client) getNextJSData(ctx context.Context, uri URI) (*nextJSData, error) {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	endpoint := url.URL{
		Scheme: "https",
		Host:   "open.spotify.com",
		Path:   fmt.Sprintf("/embed/%s/%s", uri.Type, uri.ID),
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

func (c *Client) GetTrack(ctx context.Context, uri URI) (*Track, error) {
	if uri.Type != EntityTypeTrack {
		return nil, fmt.Errorf(errPrefix+"unsupported entity type for track list: %s", uri.Type)
	}

	data, err := c.getNextJSData(ctx, URI{
		Type: EntityTypeTrack,
		ID:   uri.ID,
	})
	if err != nil {
		return nil, err
	}

	authors := make([]Author, 0, len(data.Props.PageProps.State.Data.Entity.Artists))
	for _, artist := range data.Props.PageProps.State.Data.Entity.Artists {
		authors = append(authors, Author{
			URI:  artist.URI,
			Name: strings.TrimSpace(artist.Name),
		})
	}

	return &Track{
		URI: URI{
			ID:   data.Props.PageProps.State.Data.Entity.Id,
			Type: uri.Type,
		},
		Title:    data.Props.PageProps.State.Data.Entity.Title,
		Authors:  authors,
		Duration: time.Duration(data.Props.PageProps.State.Data.Entity.Duration) * time.Millisecond,
	}, nil
}

func (c *Client) GetTrackList(ctx context.Context, uri URI) (*TrackList, error) {
	if uri.Type != EntityTypeAlbum && uri.Type != EntityTypePlaylist {
		return nil, fmt.Errorf(errPrefix+"unsupported entity type for track list: %s", uri.Type)
	}

	data, err := c.getNextJSData(ctx, uri)
	if err != nil {
		return nil, err
	}

	tracks := make([]*Track, 0, len(data.Props.PageProps.State.Data.Entity.TrackList))
	for _, partialTrack := range data.Props.PageProps.State.Data.Entity.TrackList {
		artistsNames := strings.Split(partialTrack.Subtitle, ",")

		authors := make([]Author, 0, len(artistsNames))
		for _, artistName := range artistsNames {
			artistName = strings.TrimSpace(artistName)
			if artistName == "" {
				continue
			}

			authors = append(authors, Author{
				Name: artistName,
			})
		}

		tracks = append(tracks, &Track{
			URI:      partialTrack.URI,
			Title:    partialTrack.Title,
			Authors:  authors,
			Duration: time.Duration(partialTrack.Duration) * time.Millisecond,
		})
	}

	authorsNames := strings.Split(data.Props.PageProps.State.Data.Entity.Subtitle, ",")
	authors := make([]Author, 0, len(authorsNames))
	for _, artistName := range authorsNames {
		artistName = strings.TrimSpace(artistName)
		if artistName == "" {
			continue
		}

		authors = append(authors, Author{
			Name: artistName,
		})
	}

	return &TrackList{
		URI: URI{
			Type: data.Props.PageProps.State.Data.Entity.Type,
			ID:   data.Props.PageProps.State.Data.Entity.Id,
		},
		Title:   data.Props.PageProps.State.Data.Entity.Title,
		Authors: authors,
		Tracks:  tracks,
	}, nil
}
