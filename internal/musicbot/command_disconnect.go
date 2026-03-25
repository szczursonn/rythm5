package musicbot

import "github.com/disgoorg/disgo/discord"

var disconnectChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"disconnect", "dc", "fuckoff"},
	},
	SlashMeta: &discord.SlashCommandCreate{
		Name:        "disconnect",
		Description: "Disconnect from voice channel",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	},
	Handler: func(cctx chatCommandContext) {
		s, ok := chatCommandRequireSession(cctx)
		if !ok {
			return
		}

		s.RequestDestroy(sessionDestroyReasonRequested)
		select {
		case <-s.DestroyDone():
		case <-cctx.Bot().ctx.Done():
		}
		cctx.Reply(chatCommandReply{
			Content: ":wave: **Disconnected!**",
		})
	},
}
