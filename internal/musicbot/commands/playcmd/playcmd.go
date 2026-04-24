package playcmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/media/discordattachment"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
	"github.com/szczursonn/rythm5/internal/musicbot/sessions"
)

const queryParamName = "query"

var httpLinkInQueryRegex = regexp.MustCompile(`https?:\/\/`)

type command struct {
	sessionManager   *sessions.Manager
	queryResolver    *media.QueryResolver
	attachmentSource *discordattachment.Source
}

func New(sessionManager *sessions.Manager, queryResolver *media.QueryResolver, attachmentSource *discordattachment.Source) commands.Command {
	return &command{
		sessionManager:   sessionManager,
		queryResolver:    queryResolver,
		attachmentSource: attachmentSource,
	}
}

func (c *command) ClassicAliases() []string {
	return []string{"play", "p"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return &discord.SlashCommandCreate{
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
	}
}

func (c *command) Handle(req commands.Request) {
	guildID, ok := commands.RequireGuild(req)
	if !ok {
		return
	}

	initialReplyDoneCh := make(chan struct{})
	go func() {
		defer close(initialReplyDoneCh)
		req.Reply(commands.Reply{
			Content: ":mag: **Looking up...**",
		})
	}()

	userVoiceState, err := req.Client().Rest.GetUserVoiceState(guildID, req.AuthorID(), rest.WithCtx(req.Ctx()))
	if err != nil && !rest.IsJSONErrorCode(err, rest.JSONErrorCodeUnknownVoiceState, rest.JSONErrorCodeTargetUserNotConnectedToVoice) {
		req.Logger().Error("Failed to get user voice state", slog.Any("err", err))
		<-initialReplyDoneCh
		req.Reply(commands.ReplyUnexpectedError)
		return
	}

	if err != nil || userVoiceState.ChannelID == nil {
		<-initialReplyDoneCh
		req.Reply(commands.Reply{
			Content: messages.IconUserError + " **You must be in a voice channel**",
		})
		return
	}

	s, err := c.sessionManager.GetOrCreate(guildID, req.ChannelID(), *userVoiceState.ChannelID)
	if err != nil {
		if errors.Is(err, sessions.ErrSessionLimitHit) {
			<-initialReplyDoneCh
			req.Reply(commands.Reply{
				Content: messages.IconAppError + " **Too busy right now, try again later**",
			})
		} else {
			req.Logger().Error("Failed to create session", slog.Any("err", err))
			<-initialReplyDoneCh
			req.Reply(commands.ReplyUnexpectedError)
		}

		return
	}

	var query string
	var messageReference *discord.MessageReference
	req.ExtractArgs(commands.ArgsExtractor{
		Classic: func(ev *events.MessageCreate, argsLine string) {
			query = argsLine
			messageReference = ev.Message.MessageReference
		},
		Slash: func(ev *events.ApplicationCommandInteractionCreate) {
			opt, ok := ev.SlashCommandInteractionData().Options[queryParamName]
			if ok && opt.Type == discord.ApplicationCommandOptionTypeString {
				query = strings.TrimSpace(opt.String())
			}
		},
	})

	mediaLookupCtx, cancelMediaLookupCtx := context.WithTimeout(req.Ctx(), time.Second*30)
	defer cancelMediaLookupCtx()

	var tracksToAdd []media.Track
	var playlistToAdd media.Playlist

	if query == "" {
		if messageReference == nil {
			<-initialReplyDoneCh
			req.Reply(commands.Reply{
				Content: messages.IconUserError + " **You have to provide a link or a search query**",
			})
			return
		}

		refMsg, err := req.Client().Rest.GetMessage(*messageReference.ChannelID, *messageReference.MessageID, rest.WithCtx(mediaLookupCtx))

		if err != nil {
			req.Logger().Error("Failed to fetch referenced message", slog.String("referencedMessageId", messageReference.MessageID.String()), slog.Any("err", err))
			<-initialReplyDoneCh
			req.Reply(commands.ReplyUnexpectedError)
			return
		}

		if len(refMsg.Attachments) == 0 {
			<-initialReplyDoneCh
			req.Reply(commands.Reply{
				Content: messages.IconUserError + " **The replied-to message has no attachments**",
			})
			return
		}

		tracksToAdd = append(tracksToAdd, c.attachmentSource.MakeTrack(&refMsg.Attachments[0]))
	} else {
		queryResult, err := c.queryResolver.Query(mediaLookupCtx, query)

		if err != nil {
			if errors.Is(err, media.ErrUnsupportedQuery) {
				<-initialReplyDoneCh
				req.Reply(commands.Reply{
					Content: messages.IconUserError + " **Unsupported query**",
				})
			} else {
				req.Logger().Error("Failed to query media", slog.String("query", query), slog.Any("err", err))
				<-initialReplyDoneCh
				req.Reply(commands.ReplyUnexpectedError)
			}
			return
		}

		if queryResult.Track != nil {
			tracksToAdd = append(tracksToAdd, queryResult.Track)
		} else {
			playlistToAdd = queryResult.Playlist
			tracksToAdd = playlistToAdd.Tracks()
		}
	}

	isImmediatePlayback, err := s.Enqueue(req.Ctx(), tracksToAdd...)
	if err != nil {
		req.Logger().Error("Failed to enqueue tracks", slog.Any("err", err))
		<-initialReplyDoneCh
		req.Reply(commands.ReplyUnexpectedError)
		return
	}
	<-initialReplyDoneCh

	shouldSuppressEmbeds := httpLinkInQueryRegex.MatchString(query)

	if playlistToAdd == nil {
		if isImmediatePlayback {
			req.Reply(commands.Reply{
				Content:        fmt.Sprintf(":musical_note: **Playing %s!**", messages.MakeMarkdownLink(tracksToAdd[0].Title(), tracksToAdd[0].WebpageURL())),
				SuppressEmbeds: shouldSuppressEmbeds,
			})
		} else {
			req.Reply(commands.Reply{
				Content:        fmt.Sprintf(":musical_note: **Added %s to the queue!**", messages.MakeMarkdownLink(tracksToAdd[0].Title(), tracksToAdd[0].WebpageURL())),
				SuppressEmbeds: shouldSuppressEmbeds,
			})
		}
	} else {
		req.Reply(commands.Reply{
			Content:        fmt.Sprintf(":notes: **Added %d tracks to the queue from %s!**", len(tracksToAdd), messages.MakeMarkdownLink(playlistToAdd.Title(), playlistToAdd.WebpageURL())),
			SuppressEmbeds: shouldSuppressEmbeds,
		})
	}
}
