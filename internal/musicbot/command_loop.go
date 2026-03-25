package musicbot

import "github.com/disgoorg/disgo/discord"

var loopChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"loop"},
	},
	SlashMeta: &discord.SlashCommandCreate{
		Name:        "loop",
		Description: "Toggle looping",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	},
	Handler: func(cctx chatCommandContext) {
		s, ok := chatCommandRequireSession(cctx)
		if !ok {
			return
		}

		if s.Looping() {
			s.SetLooping(false)
			cctx.Reply(chatCommandReply{
				Content: ":arrow_forward: **Looping off!**",
			})
		} else {
			s.SetLooping(true)
			cctx.Reply(chatCommandReply{
				Content: ":arrows_counterclockwise: **Looping on!**",
			})
		}
	},
}
