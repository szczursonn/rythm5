package musicbot

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

type slashChatCommandContext struct {
	bot     *Bot
	event   *events.ApplicationCommandInteractionCreate
	logger  *slog.Logger
	replied bool
}

func (cctx *slashChatCommandContext) GuildID() *snowflake.ID {
	return cctx.event.GuildID()
}

func (cctx *slashChatCommandContext) ChannelID() snowflake.ID {
	return cctx.event.Channel().ID()
}

func (cctx *slashChatCommandContext) UserID() snowflake.ID {
	return cctx.event.User().ID
}

func (cctx *slashChatCommandContext) Bot() *Bot {
	return cctx.bot
}

func (cctx *slashChatCommandContext) Logger() *slog.Logger {
	return cctx.logger
}

func (cctx *slashChatCommandContext) Reply(opts chatCommandReply) {
	flags := discord.MessageFlagSuppressNotifications
	if opts.SuppressEmbeds {
		flags = flags.Add(discord.MessageFlagSuppressEmbeds)
	}
	if opts.Ephemeral {
		flags = flags.Add(discord.MessageFlagEphemeral)
	}

	if !cctx.replied {
		if err := cctx.event.CreateMessage(discord.MessageCreate{
			Content: opts.Content,
			Embeds:  opts.Embeds,
			Flags:   flags,
		}, rest.WithCtx(cctx.bot.ctx)); err != nil {
			cctx.logger.Error("Failed to create reply message", slog.Any("err", err))
			return
		}

		cctx.replied = true
		return
	}

	if _, err := cctx.bot.client.Rest.UpdateInteractionResponse(cctx.bot.client.ApplicationID, cctx.event.Token(), discord.MessageUpdate{
		Content: &opts.Content,
		Embeds:  &opts.Embeds,
		Flags:   &flags,
	}, rest.WithCtx(cctx.bot.ctx)); err != nil {
		cctx.logger.Error("Failed to update reply message", slog.Any("err", err))
	}
}

func (b *Bot) handleApplicationCommandInteractionCreateEvent(event *events.ApplicationCommandInteractionCreate) {
	if event.Type() != discord.InteractionTypeApplicationCommand {
		return
	}

	cmd, ok := slashChatCommandByName[event.Data.CommandName()]
	if !ok {
		return
	}

	var loggerGuildID string
	if guildID := event.GuildID(); guildID != nil {
		loggerGuildID = guildID.String()
	}

	b.cmdHandlersWg.Go(func() {
		cmd.Handler(&slashChatCommandContext{
			bot:   b,
			event: event,
			logger: b.logger.With(
				slog.String("guildId", loggerGuildID),
				slog.String("channelId", event.Channel().ID().String()),
				slog.String("userId", event.User().ID.String()),
				slog.String("interactionId", event.ID().String()),
			),
		})
	})
}
