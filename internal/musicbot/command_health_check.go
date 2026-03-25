package musicbot

var healthCheckChatCommand = &chatCommand{
	ClassicMeta: &chatCommandClassicMetadata{
		Aliases: []string{"healthcheck", "hc"},
	},
	Handler: func(cctx chatCommandContext) {
		if !chatCommandRequireAdminChannel(cctx) {
			return
		}

		cctx.Reply(chatCommandReply{
			Content: ":hourglass_flowing_sand: **Running...**",
		})

		failures := cctx.Bot().runHealthChecks()
		if len(failures) == 0 {
			cctx.Reply(chatCommandReply{
				Content: ":white_check_mark: **All good!**",
			})
			return
		}

		cctx.Reply(chatCommandReply{
			Content:        createHealthCheckFailureMessage(failures),
			SuppressEmbeds: true,
		})
	},
}
