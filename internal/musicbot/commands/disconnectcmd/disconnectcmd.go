package disconnectcmd

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/sessions"
)

type command struct {
	sessionManager *sessions.Manager
}

func New(sessionManager *sessions.Manager) commands.Command {
	return &command{
		sessionManager: sessionManager,
	}
}

func (c *command) ClassicAliases() []string {
	return []string{"disconnect", "dc", "fuckoff"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return &discord.SlashCommandCreate{
		Name:        "disconnect",
		Description: "Disconnect from voice channel",
		Contexts: []discord.InteractionContextType{
			discord.InteractionContextTypeGuild,
		},
	}
}

func (c *command) Handle(req commands.Request) {
	s, ok := commands.RequireSession(req, c.sessionManager)
	if !ok {
		return
	}

	s.Destroy()
	req.Reply(commands.Reply{
		Content: ":wave: **Disconnected!**",
	})
}
