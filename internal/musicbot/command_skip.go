package musicbot

import "github.com/disgoorg/disgo/discord"

var skipChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"skip", "s", "fs"},
	},
	SlashMeta: &discord.SlashCommandCreate{
		Name:        "skip",
		Description: "Skip the current track",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	},
	Handler: func(cctx chatCommandContext) {
		s, ok := chatCommandRequireSession(cctx)
		if !ok {
			return
		}

		if s.CurrentTrack() == nil {
			cctx.Reply(chatCommandReply{
				Content:   iconUserError + " **Nothing is playing!**",
				Ephemeral: true,
			})
			return
		}

		s.Skip()
		cctx.Reply(chatCommandReply{
			Content: ":track_next: **Skipped!**",
		})
	},
}
