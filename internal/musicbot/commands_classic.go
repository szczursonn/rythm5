package musicbot

import (
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

type classicChatCommandContext struct {
	bot        *Bot
	event      *events.MessageCreate
	args       string
	logger     *slog.Logger
	replyMsgID *snowflake.ID
}

func (cctx *classicChatCommandContext) GuildID() *snowflake.ID {
	return cctx.event.GuildID
}

func (cctx *classicChatCommandContext) ChannelID() snowflake.ID {
	return cctx.event.ChannelID
}

func (cctx *classicChatCommandContext) UserID() snowflake.ID {
	return cctx.event.Message.Author.ID
}

func (cctx *classicChatCommandContext) Bot() *Bot {
	return cctx.bot
}

func (cctx *classicChatCommandContext) Logger() *slog.Logger {
	return cctx.logger
}

func (cctx *classicChatCommandContext) Reply(opts chatCommandReply) {
	flags := discord.MessageFlagSuppressNotifications
	if opts.SuppressEmbeds {
		flags = flags.Add(discord.MessageFlagSuppressEmbeds)
	}

	if cctx.replyMsgID != nil {
		_, err := cctx.bot.client.Rest.UpdateMessage(cctx.event.ChannelID, *cctx.replyMsgID, discord.MessageUpdate{
			Content: &opts.Content,
			Embeds:  &opts.Embeds,
			Flags:   &flags,
		}, rest.WithCtx(cctx.bot.ctx))
		if err != nil {
			cctx.logger.Error("Failed to update reply message", slog.Any("err", err))
		}
		return
	}

	reply, err := cctx.bot.client.Rest.CreateMessage(cctx.event.ChannelID, discord.MessageCreate{
		Content: opts.Content,
		Embeds:  opts.Embeds,
		MessageReference: &discord.MessageReference{
			MessageID: &cctx.event.MessageID,
			ChannelID: &cctx.event.ChannelID,
			GuildID:   cctx.event.GuildID,
		},
		Flags: flags,
	}, rest.WithCtx(cctx.bot.ctx))
	if err != nil {
		cctx.logger.Error("Failed to create reply message", slog.Any("err", err))
		return
	}

	cctx.replyMsgID = &reply.ID
}

func (b *Bot) handleMessageCreateEvent(event *events.MessageCreate) {
	content := strings.TrimSpace(event.Message.Content)
	if !strings.HasPrefix(content, b.classicCommandPrefix) {
		return
	}

	cmdName, args, _ := strings.Cut(strings.TrimPrefix(content, b.classicCommandPrefix), " ")
	cmdName = strings.ToLower(cmdName)
	args = strings.TrimSpace(args)

	cmd, ok := classicChatCommandByAlias[cmdName]
	if !ok {
		return
	}

	var loggerGuildID string
	if event.GuildID != nil {
		loggerGuildID = event.GuildID.String()
	}

	logger := b.logger.With(
		slog.String("guildId", loggerGuildID),
		slog.String("channelId", event.ChannelID.String()),
		slog.String("userId", event.Message.Author.ID.String()),
		slog.String("messageId", event.MessageID.String()),
		slog.String("args", args),
	)
	logger.Debug("Handling classic command")

	b.cmdHandlersWg.Go(func() {
		cmd.Handler(&classicChatCommandContext{
			bot:    b,
			event:  event,
			args:   args,
			logger: logger,
		})
	})
}
