package discordattachment

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/szczursonn/rythm5/internal/httpaudio"
	"github.com/szczursonn/rythm5/internal/media"
)

type Source struct {
	httpAudio *httpaudio.Client
}

func NewProvider(httpAudio *httpaudio.Client) *Source {
	return &Source{
		httpAudio: httpAudio,
	}
}

func (s *Source) MakeTrack(attachment *discord.Attachment) media.Track {
	t := &track{
		url: attachment.URL,
		s:   s,
	}

	if attachment.Title != nil {
		t.title = *attachment.Title
	} else {
		t.title = attachment.Filename
	}

	if attachment.DurationSecs != nil {
		t.duration = time.Duration(*attachment.DurationSecs) * time.Second
	}

	return t
}
