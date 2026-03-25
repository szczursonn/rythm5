package spotifyinfo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type EntityType string

const (
	EntityTypeTrack    EntityType = "track"
	EntityTypePlaylist EntityType = "playlist"
	EntityTypeAlbum    EntityType = "album"
	EntityTypeArtist   EntityType = "artist"
)

func (entityType EntityType) IsValid() bool {
	switch entityType {
	case EntityTypeTrack, EntityTypePlaylist, EntityTypeAlbum, EntityTypeArtist:
		return true
	}
	return false
}

type URI struct {
	ID   string
	Type EntityType
}

func (uri URI) MarshalJSON() ([]byte, error) {
	return json.Marshal(uri.String())
}

func (uri *URI) UnmarshalJSON(data []byte) error {
	str := ""
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	uriVal, err := ParseURI(str)
	if err != nil {
		return err
	}
	*uri = uriVal

	return nil
}

func (uri *URI) String() string {
	return fmt.Sprintf("spotify:%s:%s", uri.Type, uri.ID)
}

func (uri *URI) URL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "open.spotify.com",
		Path:   fmt.Sprintf("/%s/%s", uri.Type, uri.ID),
	}
}

func ParseURI(rawURI string) (URI, error) {
	uriParts := strings.Split(rawURI, ":")
	if len(uriParts) != 3 {
		return URI{}, fmt.Errorf(errPrefix+"uri has invalid amount of parts: %s", rawURI)
	}
	if uriParts[0] != "spotify" {
		return URI{}, fmt.Errorf(errPrefix+"uri does not start with \"spotify\": %s", rawURI)
	}

	entityType := EntityType(uriParts[1])
	if !entityType.IsValid() {
		return URI{}, fmt.Errorf(errPrefix+"uri has invalid entity type: %s", rawURI)
	}

	return URI{
		ID:   uriParts[2],
		Type: entityType,
	}, nil
}

func URLStringToURI(rawURL string) (URI, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return URI{}, err
	}

	return URLToURI(parsedURL)
}

func URLToURI(parsedURL *url.URL) (URI, error) {
	if parsedURL.Host != "open.spotify.com" {
		return URI{}, fmt.Errorf(errPrefix+"invalid spotify url host: %s", parsedURL.Host)
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) != 2 {
		return URI{}, fmt.Errorf(errPrefix+"invalid spotify url path parts amount: %s", parsedURL.Path)
	}

	entityType := EntityType(pathParts[0])
	if !entityType.IsValid() {
		return URI{}, fmt.Errorf(errPrefix+"invalid spotify url entity type: %s", pathParts[0])
	}

	return URI{
		ID:   pathParts[1],
		Type: entityType,
	}, nil
}
