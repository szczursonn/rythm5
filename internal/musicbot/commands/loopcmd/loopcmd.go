package loopcmd

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
	return []string{"loop"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return &discord.SlashCommandCreate{
		Name:        "loop",
		Description: "Toggle looping",
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

	if s.Looping() {
		s.SetLooping(false)
		req.Reply(commands.Reply{
			Content: ":arrow_forward: **Looping off!**",
		})
	} else {
		s.SetLooping(true)
		req.Reply(commands.Reply{
			Content: ":arrows_counterclockwise: **Looping on!**",
		})
	}
}
