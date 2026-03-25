package musicbot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/szczursonn/rythm5/internal/mdutil"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/media/mediadirecturl"
)

var playChatCommand = func() *chatCommand {
	const queryParamName = "query"

	return &chatCommand{
		ClassicMeta: &chatCommandClassicMetadata{
			Aliases: []string{"play", "p"},
		},
		SlashMeta: &discord.SlashCommandCreate{
			Name:        "play",
			Description: "Play a song or add it to the queue",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        queryParamName,
					Description: "Search query or URL",
					Required:    true,
				},
			},
			Contexts: []discord.InteractionContextType{
				discord.InteractionContextTypeGuild,
			},
		},
		Handler: func(cctx chatCommandContext) {
			guildID, ok := chatCommandRequireGuild(cctx)
			if !ok {
				return
			}

			initialReplyDoneCh := make(chan struct{})
			go func() {
				defer close(initialReplyDoneCh)
				cctx.Reply(chatCommandReply{
					Content: ":mag: **Looking up...**",
				})
			}()

			userVoiceState, err := cctx.Bot().client.Rest.GetUserVoiceState(guildID, cctx.UserID(), rest.WithCtx(cctx.Bot().ctx))
			if err != nil {
				<-initialReplyDoneCh
				cctx.Reply(replyUnexpectedError)
				cctx.Logger().Error("Failed to get user voice state", slog.Any("err", err))
				return
			}

			if userVoiceState.ChannelID == nil {
				<-initialReplyDoneCh
				cctx.Reply(chatCommandReply{
					Content:   iconUserError + " **You must be in a voice channel**",
					Ephemeral: true,
				})
				return
			}

			s := cctx.Bot().getOrCreateSession(guildID, cctx.ChannelID(), *userVoiceState.ChannelID)
			mediaLookupCtx, cancelMediaLookupCtx := context.WithTimeout(cctx.Bot().ctx, time.Second*15)
			defer cancelMediaLookupCtx()

			var query string
			var messageReference *discord.MessageReference
			switch cctx := cctx.(type) {
			case *classicChatCommandContext:
				query = cctx.args
				messageReference = cctx.event.Message.MessageReference
			case *slashChatCommandContext:
				opt, ok := cctx.event.SlashCommandInteractionData().Options[queryParamName]
				if ok && opt.Type == discord.ApplicationCommandOptionTypeString {
					query = strings.TrimSpace(opt.String())
				}
			}

			if query == "" {
				if messageReference != nil {
					refMsg, err := cctx.Bot().client.Rest.GetMessage(*messageReference.ChannelID, *messageReference.MessageID, rest.WithCtx(mediaLookupCtx))
					<-initialReplyDoneCh
					if err != nil {
						cctx.Reply(replyUnexpectedError)
						cctx.Logger().Error("Failed to fetch referenced message", slog.String("referencedMessageId", messageReference.MessageID.String()), slog.Any("err", err))
						return
					}

					if len(refMsg.Attachments) == 0 {
						cctx.Reply(chatCommandReply{
							Content:   iconUserError + " **The replied-to message has no attachments**",
							Ephemeral: true,
						})
						return
					}

					track := &mediadirecturl.DirectURLTrack{
						Title: refMsg.Attachments[0].Filename,
						URL:   refMsg.Attachments[0].URL,
					}
					cctx.Reply(chatCommandReply{
						Content: fmt.Sprintf(":musical_note: **Added %s to the queue!**", mdutil.MakeLink(track.GetTitle(), track.GetURL())),
					})
					s.Enqueue(track)
					return
				}

				<-initialReplyDoneCh
				cctx.Reply(chatCommandReply{
					Content:   iconUserError + " **You have to provide a link or a search query**",
					Ephemeral: true,
				})
				return
			}

			if queryURL, err := url.ParseRequestURI(query); err == nil {
				queryResult, err := cctx.Bot().mediaProvider.QueryByURL(mediaLookupCtx, queryURL, media.URLQueryOptions{
					Preference: media.QueryPreferenceTrack,
				})
				<-initialReplyDoneCh
				if err != nil {
					if errors.Is(err, media.ErrUnsupportedQuery) {
						cctx.Reply(chatCommandReply{
							Content: iconUserError + " **Unsupported link**",
						})
					} else {
						cctx.Reply(replyUnexpectedError)
						cctx.Logger().Error("Failed to query media by url", slog.String("query", queryURL.String()), slog.Any("err", err))
					}

					return
				}

				if queryResult.Track != nil {
					s.Enqueue(queryResult.Track)
					cctx.Reply(chatCommandReply{
						Content:        fmt.Sprintf(":musical_note: **Added %s to the queue!**", mdutil.MakeLink(queryResult.Track.GetTitle(), queryResult.Track.GetURL())),
						SuppressEmbeds: true,
					})
				} else {
					tracks := queryResult.Playlist.GetTracks()
					s.Enqueue(tracks...)
					cctx.Reply(chatCommandReply{
						Content:        fmt.Sprintf(":notes: **Added %d tracks to the queue from %s!**", len(tracks), mdutil.MakeLink(queryResult.Playlist.GetTitle(), queryResult.Playlist.GetURL())),
						SuppressEmbeds: true,
					})
				}
			} else {
				tracks, err := cctx.Bot().mediaProvider.QueryBySearch(mediaLookupCtx, query, media.SearchQueryOptions{
					MaxResults: 1,
				})
				<-initialReplyDoneCh
				if err != nil {
					cctx.Reply(replyUnexpectedError)
					cctx.Logger().Error("Failed to query media by search", slog.String("query", query), slog.Any("err", err))
					return
				}

				if len(tracks) == 0 {
					cctx.Reply(chatCommandReply{
						Content: iconUserError + " **No results found**",
					})
					return
				}

				s.Enqueue(tracks[0])
				cctx.Reply(chatCommandReply{
					Content: fmt.Sprintf(":musical_note: **Added %s to the queue!**", mdutil.MakeLink(tracks[0].GetTitle(), tracks[0].GetURL())),
				})
			}
		},
	}
}()
