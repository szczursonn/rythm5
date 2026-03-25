package musicbot

import (
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/szczursonn/rythm5/internal/mdutil"
	"github.com/szczursonn/rythm5/internal/media"
)

var queueChatCommand = func() *chatCommand {
	const maxVisibleItems = 5

	createEmbedTrackDescription := func(track media.Track) string {
		var sb strings.Builder

		if trackURL := track.GetURL(); trackURL != "" {
			sb.WriteString("[Link](")
			sb.WriteString(trackURL)
			sb.WriteString(") | ")
		}

		if dur := track.GetDuration(); dur > 0 {
			sb.WriteString(dur.String())
		} else {
			sb.WriteString("unknown duration")
		}

		return sb.String()
	}

	return &chatCommand{
		ClassicMeta: &chatCommandClassicMetadata{
			Aliases: []string{"queue", "q"},
		},
		SlashMeta: &discord.SlashCommandCreate{
			Name:        "queue",
			Description: "Show the current queue",
			Contexts: []discord.InteractionContextType{
				discord.InteractionContextTypeGuild,
			},
		},
		Handler: func(cctx chatCommandContext) {
			s, ok := chatCommandRequireSession(cctx)
			if !ok {
				return
			}

			embed := discord.NewEmbedBuilder().SetTitle("Queue")
			if currentTrack := s.CurrentTrack(); currentTrack == nil {
				embed.AddField("Empty", "\u2800", false)
			} else {
				embed.AddField(fmt.Sprintf("__Playing now__: %s", mdutil.EscapeMarkdown(currentTrack.GetTitle())), createEmbedTrackDescription(currentTrack), false)

				var totalDuration time.Duration
				queue := s.Queue()
				for i, track := range queue {
					if dur := track.GetDuration(); dur > 0 {
						totalDuration += dur
					}

					if i < maxVisibleItems {
						embed.AddField(fmt.Sprintf("%d. %s", i+1, mdutil.EscapeMarkdown(track.GetTitle())), createEmbedTrackDescription(track), false)
					} else if i == maxVisibleItems {
						embed.AddField(fmt.Sprintf("and %d more queued!", len(queue)-maxVisibleItems), "\u2800", false)
					}
				}

				embed.SetDescription(fmt.Sprintf("Queue length: **%s**", totalDuration))
			}

			cctx.Reply(chatCommandReply{
				Embeds: []discord.Embed{
					embed.Build(),
				},
			})
		},
	}
}()
