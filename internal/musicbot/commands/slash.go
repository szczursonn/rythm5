package commands

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func (d *Dispatcher) handleApplicationCommandInteractionEvent(ev *events.ApplicationCommandInteractionCreate) {
	if ev.Type() != discord.InteractionTypeApplicationCommand {
		return
	}

	cmd, ok := d.slashByName[ev.Data.CommandName()]
	if !ok {
		return
	}

	var loggerGuildID string
	if guildID := ev.GuildID(); guildID != nil {
		loggerGuildID = guildID.String()
	}

	d.handlersWg.Go(func() {
		cmd.Handle(&slashRequest{
			ev:  ev,
			ctx: d.ctx,
			logger: d.logger.With(
				slog.String("guildID", loggerGuildID),
				slog.String("channelID", ev.Channel().ID().String()),
				slog.String("userID", ev.User().ID.String()),
				slog.String("interactionID", ev.ID().String()),
			),
		})
	})
}

type slashRequest struct {
	ev      *events.ApplicationCommandInteractionCreate
	ctx     context.Context
	logger  *slog.Logger
	replied bool
}

func (sreq *slashRequest) GuildID() *snowflake.ID {
	return sreq.ev.GuildID()
}

func (sreq *slashRequest) ChannelID() snowflake.ID {
	return sreq.ev.Channel().ID()
}

func (sreq *slashRequest) AuthorID() snowflake.ID {
	return sreq.ev.User().ID
}

func (sreq *slashRequest) Client() *bot.Client {
	return sreq.ev.Client()
}

func (sreq *slashRequest) Ctx() context.Context {
	return sreq.ctx
}

func (sreq *slashRequest) Logger() *slog.Logger {
	return sreq.logger
}

func (sreq *slashRequest) ExtractArgs(ex ArgsExtractor) {
	if ex.Slash != nil {
		ex.Slash(sreq.ev)
	}
}

func (sreq *slashRequest) Reply(opts Reply) {
	flags := discord.MessageFlagSuppressNotifications
	if opts.SuppressEmbeds {
		flags = flags.Add(discord.MessageFlagSuppressEmbeds)
	}

	if !sreq.replied {
		if err := sreq.ev.CreateMessage(discord.MessageCreate{
			Content: opts.Content,
			Embeds:  opts.Embeds,
			Flags:   flags,
		}, rest.WithCtx(sreq.ctx)); err != nil {
			sreq.logger.Error("Failed to create reply message", slog.Any("err", err))
			return
		}

		sreq.replied = true
		return
	}

	client := sreq.ev.Client()

	if _, err := client.Rest.UpdateInteractionResponse(client.ApplicationID, sreq.ev.Token(), discord.MessageUpdate{
		Content: &opts.Content,
		Embeds:  &opts.Embeds,
		Flags:   &flags,
	}, rest.WithCtx(sreq.ctx)); err != nil {
		sreq.logger.Error("Failed to update reply message", slog.Any("err", err))
	}
}
