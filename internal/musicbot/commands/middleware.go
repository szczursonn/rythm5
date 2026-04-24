package commands

import (
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
	"github.com/szczursonn/rythm5/internal/musicbot/sessions"
)

func RequireGuild(req Request) (snowflake.ID, bool) {
	guildID := req.GuildID()
	if guildID == nil {
		req.Reply(Reply{
			Content: messages.IconUserError + " **This command can only be used in a server**",
		})
		return 0, false
	}

	return *guildID, true
}

func RequireSession(req Request, sessionManager *sessions.Manager) (*sessions.Session, bool) {
	guildID, ok := RequireGuild(req)
	if !ok {
		return nil, false
	}

	s := sessionManager.Get(guildID)
	if s == nil {
		req.Reply(Reply{
			Content: messages.IconUserError + " **I am not active in this server**",
		})
		return nil, false
	}

	return s, true
}

func RequireAdminChannel(req Request, adminChannelID snowflake.ID) bool {
	if req.ChannelID() != adminChannelID {
		req.Reply(Reply{
			Content: messages.IconUserError + " **Unavailable**",
		})
		return false
	}

	return true
}
