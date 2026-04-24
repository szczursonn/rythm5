package ytdlp

import "github.com/szczursonn/rythm5/internal/media"

type playlist struct {
	title  string
	url    string
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

func (qs *querySource) extractPlaylist(yr *ytdlpResult) *playlist {
	p := &playlist{
		title:  yr.Title,
		url:    yr.WebpageURL,
		tracks: make([]media.Track, 0, len(yr.Entries)),
	}

	for _, entry := range yr.Entries {
		t, err := qs.extractTrack(&entry)
		if err == nil {
			p.tracks = append(p.tracks, t)
		}
	}

	return p
}
