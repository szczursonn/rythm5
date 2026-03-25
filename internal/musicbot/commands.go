package musicbot

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const (
	iconUserError = ":x:"
	iconAppError  = ":octagonal_sign:"
)

var (
	replyUnexpectedError = chatCommandReply{
		Content:   iconAppError + " **An unexpected error has occured**",
		Ephemeral: true,
	}
)

type chatCommand struct {
	ClassicMeta *chatCommandClassicMetadata
	SlashMeta   *discord.SlashCommandCreate
	Handler     func(cctx chatCommandContext)
}

type chatCommandClassicMetadata struct {
	Aliases []string
}

type chatCommandReply struct {
	Content        string
	Embeds         []discord.Embed
	Ephemeral      bool
	SuppressEmbeds bool
}

type chatCommandContext interface {
	GuildID() *snowflake.ID
	ChannelID() snowflake.ID
	UserID() snowflake.ID
	Bot() *Bot
	Logger() *slog.Logger
	Reply(opts chatCommandReply)
}

func chatCommandRequireGuild(cctx chatCommandContext) (snowflake.ID, bool) {
	guildID := cctx.GuildID()
	if guildID == nil {
		cctx.Reply(chatCommandReply{
			Content:   iconUserError + " **This command can only be used in a server**",
			Ephemeral: true,
		})
		return 0, false
	}

	return *guildID, true
}

func chatCommandRequireSession(cctx chatCommandContext) (*session, bool) {
	guildID, ok := chatCommandRequireGuild(cctx)
	if !ok {
		return nil, false
	}

	s := cctx.Bot().getSession(guildID)
	if s == nil {
		cctx.Reply(chatCommandReply{
			Content:   iconUserError + " **I am not active in this server**",
			Ephemeral: true,
		})
		return nil, false
	}

	return s, true
}

func chatCommandRequireAdminChannel(cctx chatCommandContext) bool {
	adminChannelID := cctx.Bot().adminChannelID
	if adminChannelID == nil || cctx.ChannelID() != *adminChannelID {
		cctx.Reply(chatCommandReply{
			Content:   iconUserError + " **Unavailable**",
			Ephemeral: true,
		})
		return false
	}

	return true
}

var classicChatCommandByAlias, slashChatCommandByName = func() (map[string]*chatCommand, map[string]*chatCommand) {
	commands := []*chatCommand{
		playChatCommand,
		queueChatCommand,
		skipChatCommand,
		clearChatCommand,
		loopChatCommand,
		shuffleChatCommand,
		disconnectChatCommand,
		healthCheckChatCommand,
	}
	slashDefinitions := make([]discord.ApplicationCommandCreate, 0, len(commands))
	for _, cmd := range commands {
		if cmd.SlashMeta != nil {
			slashDefinitions = append(slashDefinitions, cmd.SlashMeta)
		}
	}
	commands = append(commands, initSlashChatCommand(slashDefinitions))

	classicAliasToCmd := map[string]*chatCommand{}
	slashNameToCmd := map[string]*chatCommand{}

	for _, cmd := range commands {
		if cmd.ClassicMeta != nil {
			for _, classicAlias := range cmd.ClassicMeta.Aliases {
				classicAliasToCmd[classicAlias] = cmd
			}
		}

		if cmd.SlashMeta != nil {
			slashNameToCmd[cmd.SlashMeta.Name] = cmd
		}
	}

	return classicAliasToCmd, slashNameToCmd
}()
