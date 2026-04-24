package commands

import (
	"context"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func (d *Dispatcher) handleMessageCreateEvent(ev *events.MessageCreate) {
	content := strings.TrimSpace(ev.Message.Content)
	if !strings.HasPrefix(content, d.classicPrefix) {
		return
	}

	cmdAlias, argsLine, _ := strings.Cut(strings.TrimPrefix(content, d.classicPrefix), " ")
	cmdAlias = strings.ToLower(cmdAlias)
	argsLine = strings.TrimSpace(argsLine)

	cmd, ok := d.classicByAlias[cmdAlias]
	if !ok {
		return
	}

	var loggerGuildID string
	if ev.GuildID != nil {
		loggerGuildID = ev.GuildID.String()
	}

	logger := d.logger.With(
		slog.String("guildID", loggerGuildID),
		slog.String("channelID", ev.ChannelID.String()),
		slog.String("userID", ev.Message.Author.ID.String()),
		slog.String("messageID", ev.MessageID.String()),
		slog.String("argsLine", argsLine),
	)
	logger.Debug("Handling classic command")

	d.handlersWg.Go(func() {
		cmd.Handle(&classicRequest{
			ev:       ev,
			argsLine: argsLine,
			ctx:      d.ctx,
			logger:   logger,
		})
	})
}

type classicRequest struct {
	ev         *events.MessageCreate
	argsLine   string
	ctx        context.Context
	logger     *slog.Logger
	replyMsgID *snowflake.ID
}

func (creq *classicRequest) GuildID() *snowflake.ID {
	return creq.ev.GuildID
}

func (creq *classicRequest) ChannelID() snowflake.ID {
	return creq.ev.ChannelID
}

func (creq *classicRequest) AuthorID() snowflake.ID {
	return creq.ev.Message.Author.ID
}

func (creq *classicRequest) Client() *bot.Client {
	return creq.ev.Client()
}

func (creq *classicRequest) Ctx() context.Context {
	return creq.ctx
}

func (creq *classicRequest) Logger() *slog.Logger {
	return creq.logger
}

func (creq *classicRequest) ExtractArgs(ex ArgsExtractor) {
	if ex.Classic != nil {
		ex.Classic(creq.ev, creq.argsLine)
	}
}

func (creq *classicRequest) Reply(opts Reply) {
	flags := discord.MessageFlagSuppressNotifications
	if opts.SuppressEmbeds {
		flags = flags.Add(discord.MessageFlagSuppressEmbeds)
	}

	if creq.replyMsgID != nil {
		_, err := creq.ev.Client().Rest.UpdateMessage(creq.ev.ChannelID, *creq.replyMsgID, discord.MessageUpdate{
			Content: &opts.Content,
			Embeds:  &opts.Embeds,
			Flags:   &flags,
		}, rest.WithCtx(creq.ctx))
		if err != nil {
			creq.logger.Error("Failed to update reply message", slog.Any("err", err))
		}
		return
	}

	reply, err := creq.ev.Client().Rest.CreateMessage(creq.ev.ChannelID, discord.MessageCreate{
		Content: opts.Content,
		Embeds:  opts.Embeds,
		MessageReference: &discord.MessageReference{
			MessageID: &creq.ev.MessageID,
			ChannelID: &creq.ev.ChannelID,
			GuildID:   creq.ev.GuildID,
		},
		Flags: flags,
	}, rest.WithCtx(creq.ctx))
	if err != nil {
		creq.logger.Error("Failed to create reply message", slog.Any("err", err))
		return
	}

	creq.replyMsgID = &reply.ID
}
