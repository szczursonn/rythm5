package musicbot

import "github.com/disgoorg/disgo/discord"

var clearChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"clear"},
	},
	SlashMeta: &discord.SlashCommandCreate{
		Name:        "clear",
		Description: "Clear the queue",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	},
	Handler: func(cctx chatCommandContext) {
		s, ok := chatCommandRequireSession(cctx)
		if !ok {
			return
		}

		s.ClearQueue()
		cctx.Reply(chatCommandReply{
			Content: ":broom: **Queue cleared!**",
		})
	},
}
