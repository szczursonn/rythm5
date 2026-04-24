package commands

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type Command interface {
	ClassicAliases() []string
	SlashDef() *discord.SlashCommandCreate
	Handle(req Request)
}

type Request interface {
	GuildID() *snowflake.ID
	ChannelID() snowflake.ID
	AuthorID() snowflake.ID
	Client() *bot.Client
	Ctx() context.Context
	Logger() *slog.Logger
	ExtractArgs(ex ArgsExtractor)
	Reply(r Reply)
}

type Reply struct {
	Content        string
	Embeds         []discord.Embed
	SuppressEmbeds bool
}

type ArgsExtractor struct {
	Classic func(ev *events.MessageCreate, argsLine string)
	Slash   func(ev *events.ApplicationCommandInteractionCreate)
}
