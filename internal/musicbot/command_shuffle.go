package musicbot

import "github.com/disgoorg/disgo/discord"

var shuffleChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"shuffle"},
	},
	SlashMeta: &discord.SlashCommandCreate{
		Name:        "shuffle",
		Description: "Shuffle the queue",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	},
	Handler: func(cctx chatCommandContext) {
		s, ok := chatCommandRequireSession(cctx)
		if !ok {
			return
		}

		s.ShuffleQueue()
		cctx.Reply(chatCommandReply{
			Content: ":twisted_rightwards_arrows: **Shuffled!**",
		})
	},
}
