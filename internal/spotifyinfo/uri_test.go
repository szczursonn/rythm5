package spotifyinfo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/szczursonn/rythm5/internal/spotifyinfo"
)

func TestURLParse(t *testing.T) {
	tests := []struct {
		RawURL             string
		ExpectedEntityType spotifyinfo.EntityType
		ExpectedID         string
	}{
		{
			RawURL:             "https://open.spotify.com/track/123?si=abc321",
			ExpectedEntityType: spotifyinfo.EntityTypeTrack,
			ExpectedID:         "123",
		},
		{
			RawURL:             "https://open.spotify.com/track/123",
			ExpectedEntityType: spotifyinfo.EntityTypeTrack,
			ExpectedID:         "123",
		},
		{
			RawURL:             "https://open.spotify.com/album/321abc",
			ExpectedEntityType: spotifyinfo.EntityTypeAlbum,
			ExpectedID:         "321abc",
		},
		{
			RawURL:             "https://open.spotify.com/playlist/xa7483bdcax8as890asy8gdabvdasxdd",
			ExpectedEntityType: spotifyinfo.EntityTypePlaylist,
			ExpectedID:         "xa7483bdcax8as890asy8gdabvdasxdd",
		},
		{
			RawURL:             "https://open.spotify.com/artist/rrarrarasrarsa",
			ExpectedEntityType: spotifyinfo.EntityTypeArtist,
			ExpectedID:         "rrarrarasrarsa",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d (%s)", i, test.RawURL), func(t *testing.T) {
			t.Parallel()

			uri, err := spotifyinfo.URLStringToURI(test.RawURL)
			if err != nil {
				t.Error("failed to parse url", err)
			} else {
				if uri.Type != test.ExpectedEntityType {
					t.Errorf("expected entity type %s, got %s", test.ExpectedEntityType, uri.Type)
				}

				if uri.ID != test.ExpectedID {
					t.Errorf("expected id %s, got %s", test.ExpectedID, uri.ID)
				}
			}
		})
	}
}

func TestURLParseError(t *testing.T) {
	badUrls := []string{
		// Invalid entity type
		"https://open.spotify.com/skibidi/123?si=abc321",
		// Invalid host
		"https://spotify.com/track/123?si=abc321",
	}

	for i, badUrl := range badUrls {
		t.Run(fmt.Sprintf("%d (%s)", i, badUrl), func(t *testing.T) {
			t.Parallel()

			uri, err := spotifyinfo.URLStringToURI(badUrl)
			if err == nil {
				t.Fatalf("expected error, got %v", uri)
			}
		})
	}
}

func TestURIParse(t *testing.T) {
	tests := []struct {
		RawURI             string
		ExpectedEntityType spotifyinfo.EntityType
		ExpectedID         string
	}{
		{
			RawURI:             "spotify:track:123",
			ExpectedEntityType: spotifyinfo.EntityTypeTrack,
			ExpectedID:         "123",
		},
		{
			RawURI:             "spotify:album:321abc",
			ExpectedEntityType: spotifyinfo.EntityTypeAlbum,
			ExpectedID:         "321abc",
		},
		{
			RawURI:             "spotify:playlist:xa7483bdcax8as890asy8gdabvdasxdd",
			ExpectedEntityType: spotifyinfo.EntityTypePlaylist,
			ExpectedID:         "xa7483bdcax8as890asy8gdabvdasxdd",
		},
		{
			RawURI:             "spotify:artist:123",
			ExpectedEntityType: spotifyinfo.EntityTypeArtist,
			ExpectedID:         "123",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d (%s)", i, test.RawURI), func(t *testing.T) {
			t.Parallel()

			parsedURI, err := spotifyinfo.ParseURI(test.RawURI)
			if err != nil {
				t.Error("failed to parse uri", err)
			} else {
				if parsedURI.Type != test.ExpectedEntityType {
					t.Errorf("expected entity type %s, got %s", test.ExpectedEntityType, parsedURI.Type)
				}

				if parsedURI.ID != test.ExpectedID {
					t.Errorf("expected id %s, got %s", test.ExpectedID, parsedURI.ID)
				}
			}
		})
	}
}

func TestURIParseError(t *testing.T) {
	badURIs := []string{
		// Invalid entity type
		"spotify:skibidi:123",
		// Invalid first part
		"spotifity:track:123",
		// Invalid amount of parts
		"track:123",
	}

	for i, badURI := range badURIs {
		t.Run(fmt.Sprintf("%d (%s)", i, badURI), func(t *testing.T) {
			t.Parallel()

			uri, err := spotifyinfo.ParseURI(badURI)
			if err == nil {
				t.Fatalf("expected error, got %v", uri)
			}
		})
	}
}

func TestConsistentJSON(t *testing.T) {
	uri, err := spotifyinfo.URLStringToURI("https://open.spotify.com/track/123")
	if err != nil {
		t.Fatal(err)
	}

	marshalledURI, err := json.Marshal(uri)
	if err != nil {
		t.Fatal(err)
	}

	unmarshaledURI := spotifyinfo.URI{}
	if err := json.Unmarshal(marshalledURI, &unmarshaledURI); err != nil {
		t.Fatal(err, "\nmarshalled:", string(marshalledURI))
	}

	if !reflect.DeepEqual(uri, unmarshaledURI) {
		t.Fatalf("uri is different after marshalling and unmarshalling: original \"%s\", jsonned \"%s\"", uri.String(), unmarshaledURI.String())
	}
}

func TestConsistentString(t *testing.T) {
	uri, err := spotifyinfo.URLStringToURI("https://open.spotify.com/track/123")
	if err != nil {
		t.Fatal(err)
	}

	uriString := uri.String()
	parsedURI, err := spotifyinfo.ParseURI(uriString)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(uri, parsedURI) {
		t.Fatalf("uri is different after stringifying and parsing: original \"%s\", parsed \"%s\"", uri.String(), parsedURI.String())
	}
}

func TestURIToURL(t *testing.T) {
	tests := []struct {
		URI         spotifyinfo.URI
		ExpectedURL string
	}{
		{
			URI: spotifyinfo.URI{
				ID:   "123",
				Type: spotifyinfo.EntityTypeArtist,
			},
			ExpectedURL: "https://open.spotify.com/artist/123",
		},
		{
			URI: spotifyinfo.URI{
				ID:   "321",
				Type: spotifyinfo.EntityTypePlaylist,
			},
			ExpectedURL: "https://open.spotify.com/playlist/321",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d (%s)", i, test.URI), func(t *testing.T) {
			t.Parallel()

			u := test.URI.URL().String()
			if u != test.ExpectedURL {
				t.Errorf("expected url %s, got %s", test.ExpectedURL, u)
			}
		})
	}
}
