package healthcheckcmd

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/healthcheck"
)

type command struct {
	healthCheckRunner *healthcheck.Runner
	adminChannelID    snowflake.ID
}

func New(healthCheckRunner *healthcheck.Runner, adminChannelID snowflake.ID) commands.Command {
	return &command{
		healthCheckRunner: healthCheckRunner,
		adminChannelID:    adminChannelID,
	}
}

func (c *command) ClassicAliases() []string {
	return []string{"healthcheck", "hc"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return nil
}

func (c *command) Handle(req commands.Request) {
	if !commands.RequireAdminChannel(req, c.adminChannelID) {
		return
	}

	req.Reply(commands.Reply{
		Content: ":hourglass_flowing_sand: **Running...**",
	})

	failures := c.healthCheckRunner.Run(req.Ctx())
	if len(failures) == 0 {
		req.Reply(commands.Reply{
			Content: ":white_check_mark: **All good!**",
		})
		return
	}

	req.Reply(commands.Reply{
		Content:        healthcheck.MakeFailureMessage(failures),
		SuppressEmbeds: true,
	})
}
