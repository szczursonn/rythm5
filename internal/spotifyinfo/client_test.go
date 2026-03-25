package spotifyinfo_test

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"

	"github.com/szczursonn/rythm5/internal/spotifyinfo"
)

type mockRoundTripper struct {
	handler func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.handler(req)
}

type testTrack struct {
	URL             string
	Title           string
	DurationSeconds int
	AuthorNames     []string
}

func compareExpectedAndActualTrack(t *testing.T, expected *testTrack, actual *spotifyinfo.Track) {
	if actual.Title != expected.Title {
		t.Fatalf("expected track name %s, got %s", expected.Title, actual.Title)
	}

	trackDurationSeconds := int(actual.Duration.Seconds())
	if math.Abs(float64(trackDurationSeconds)-float64(expected.DurationSeconds)) > 1 {
		t.Fatalf("expected track duration %ds, got %ds", expected.DurationSeconds, trackDurationSeconds)
	}

	if len(actual.Authors) != len(expected.AuthorNames) {
		t.Fatalf("expected %d authors, got %d", len(expected.AuthorNames), len(actual.Authors))
	}

	for _, expectedAuthorName := range expected.AuthorNames {
		found := false
		for _, author := range actual.Authors {
			if author.Name == expectedAuthorName {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected author %s not found", expectedAuthorName)
		}
	}
}

func TestGetTrackReal(t *testing.T) {
	tests := []*testTrack{
		{
			URL:             "https://open.spotify.com/track/2Cncu6bEEKwbGhNcMigcEn",
			Title:           "Explosion",
			DurationSeconds: 4*60 + 10,
			AuthorNames:     []string{"Kalwi & Remi"},
		},
		{
			URL:             "https://open.spotify.com/track/54HoJi9KbLiFk1sGUQT5NJ?si=2d93254fb3e4eb1",
			Title:           "Wschód (lubię zapierdalać)",
			DurationSeconds: 4*60 + 8,
			AuthorNames:     []string{"Bedoes 2115", "Lanek", "Kosa", "White 2115"},
		},
	}

	client := spotifyinfo.Client{}
	for i, testCase := range tests {
		t.Run(fmt.Sprintf("%d (%s)", i, testCase.URL), func(t *testing.T) {
			t.Parallel()

			uri, err := spotifyinfo.URLStringToURI(testCase.URL)
			if err != nil {
				t.Fatal(err)
			}

			if uri.Type != spotifyinfo.EntityTypeTrack {
				t.Fatalf("expected entity type track, got %s", uri.Type)
			}

			track, err := client.GetTrack(context.Background(), uri)
			if err != nil {
				t.Fatal(err)
			}

			compareExpectedAndActualTrack(t, testCase, track)
		})
	}
}

type testTrackList struct {
	URL         string
	Title       string
	Type        spotifyinfo.EntityType
	FirstTracks []*testTrack
}

func compareExpectedAndActualTrackList(t *testing.T, expected *testTrackList, actual *spotifyinfo.TrackList) {
	if actual.Title != expected.Title {
		t.Fatalf("expected track list name %s, got %s", expected.Title, actual.Title)
	}

	if len(actual.Tracks) < len(expected.FirstTracks) {
		t.Fatalf("expected at least %d tracks, got %d", len(expected.FirstTracks), len(actual.Tracks))
	}

	for i, expectedTrack := range expected.FirstTracks {
		compareExpectedAndActualTrack(t, expectedTrack, actual.Tracks[i])
	}
}

func TestGetTrackListReal(t *testing.T) {
	tests := []*testTrackList{
		{
			URL:   "https://open.spotify.com/album/2ivOxIKDHxEo6WMD9m3ytn",
			Title: "I Want to Die In New Orleans",
			Type:  spotifyinfo.EntityTypeAlbum,
			FirstTracks: []*testTrack{
				{
					Title:           "King Tulip",
					DurationSeconds: 3*60 + 5,
					AuthorNames:     []string{"$uicideboy$"},
				},
				{
					Title:           "Bring out Your Dead",
					DurationSeconds: 1*60 + 46,
					AuthorNames:     []string{"$uicideboy$"},
				},
			},
		},
		{
			URL:   "https://open.spotify.com/playlist/0jWG9H3CXOUeBQ5mdzfbaA",
			Title: "Melanż u sołtysa",
			Type:  spotifyinfo.EntityTypePlaylist,
			FirstTracks: []*testTrack{
				{
					Title:           "Cheri Cheri Lady",
					DurationSeconds: 3*60 + 46,
					AuthorNames:     []string{"Modern Talking"},
				},
				{
					Title:           "Touch By Touch - Touch Maxi Version",
					DurationSeconds: 5*60 + 31,
					AuthorNames:     []string{"Joy"},
				},
				{
					Title:           "No Guidance (feat. Drake)",
					DurationSeconds: 4*60 + 21,
					AuthorNames:     []string{"Chris Brown", "Drake"},
				},
			},
		},
	}

	client := spotifyinfo.Client{}
	for i, testCase := range tests {
		t.Run(fmt.Sprintf("%d (%s)", i, testCase.URL), func(t *testing.T) {
			t.Parallel()

			uri, err := spotifyinfo.URLStringToURI(testCase.URL)
			if err != nil {
				t.Fatal(err)
			}

			if uri.Type != testCase.Type {
				t.Fatalf("expected entity type %s, got %s", testCase.Type, uri.Type)
			}

			trackList, err := client.GetTrackList(context.Background(), uri)
			if err != nil {
				t.Fatal(err)
			}

			compareExpectedAndActualTrackList(t, testCase, trackList)
		})
	}
}

func TestGetTrackListBadEntityType(t *testing.T) {
	client := spotifyinfo.Client{
		HTTPClient: &http.Client{
			Transport: &mockRoundTripper{
				handler: func(req *http.Request) (*http.Response, error) {
					t.Fatal("unexpected http request")
					return nil, nil
				},
			},
		},
	}

	_, err := client.GetTrackList(context.Background(), spotifyinfo.URI{
		Type: spotifyinfo.EntityTypeTrack,
		ID:   "123",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
