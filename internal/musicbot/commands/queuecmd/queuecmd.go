package queuecmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
	"github.com/szczursonn/rythm5/internal/musicbot/sessions"
)

const maxVisibleItems = 5

type command struct {
	sessions *sessions.Manager
}

func New(smgr *sessions.Manager) commands.Command {
	return &command{
		sessions: smgr,
	}
}

func (c *command) ClassicAliases() []string {
	return []string{"queue", "q"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return &discord.SlashCommandCreate{
		Name:        "queue",
		Description: "Show the current queue",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	}
}

func createEmbedTrackDescription(track media.Track) string {
	var sb strings.Builder

	if trackWebpageURL := track.WebpageURL(); trackWebpageURL != "" {
		sb.WriteString("[Link](")
		sb.WriteString(trackWebpageURL)
		sb.WriteString(") | ")
	}

	if dur := track.EstimatedDuration(); dur > 0 {
		sb.WriteString(dur.String())
	} else {
		sb.WriteString("unknown duration")
	}

	return sb.String()
}

func (c *command) Handle(req commands.Request) {
	s, ok := commands.RequireSession(req, c.sessions)
	if !ok {
		return
	}

	embed := discord.NewEmbed()
	embed.Title = "Queue"
	if currentTrack := s.CurrentTrack(); currentTrack == nil {
		embed = embed.AddField("Empty", "\u2800", false)
	} else {
		embed = embed.AddField(fmt.Sprintf("__Playing now__: %s", messages.EscapeMarkdown(currentTrack.Title())), createEmbedTrackDescription(currentTrack), false)

		var totalDuration time.Duration
		queue := s.Queue()
		for i, track := range queue {
			if dur := track.EstimatedDuration(); dur > 0 {
				totalDuration += dur
			}

			if i < maxVisibleItems {
				embed = embed.AddField(fmt.Sprintf("%d. %s", i+1, messages.EscapeMarkdown(track.Title())), createEmbedTrackDescription(track), false)
			} else if i == maxVisibleItems {
				embed = embed.AddField(fmt.Sprintf("and %d more queued!", len(queue)-maxVisibleItems), "\u2800", false)
			}
		}

		embed.Description = fmt.Sprintf("Queue length: **%s**", totalDuration)
	}

	req.Reply(commands.Reply{
		Embeds: []discord.Embed{
			embed,
		},
	})
}
