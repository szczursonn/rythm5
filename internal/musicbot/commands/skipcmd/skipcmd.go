package skipcmd

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
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
	return []string{"skip", "s", "fs"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return &discord.SlashCommandCreate{
		Name:        "skip",
		Description: "Skip the current track",
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

	s.SetLooping(false)

	skipped, err := s.Skip(req.Ctx())
	if err != nil {
		req.Reply(commands.ReplyUnexpectedError)
		return
	}

	if skipped {
		req.Reply(commands.Reply{
			Content: ":track_next: **Skipped!**",
		})
	} else {
		req.Reply(commands.Reply{
			Content: messages.IconUserError + " **Nothing to skip!**",
		})
	}
}
