package slashcmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
)

const (
	operationTypeRegister   = "register"
	operationTypeUnregister = "unregister"
	operationTypePrint      = "print"
)

type command struct {
	defs           []discord.ApplicationCommandCreate
	adminChannelID snowflake.ID
}

func New(cmds []commands.Command, adminChannelID snowflake.ID) commands.Command {
	defs := make([]discord.ApplicationCommandCreate, 0, len(cmds))
	for _, cmd := range cmds {
		if def := cmd.SlashDef(); def != nil {
			defs = append(defs, def)
		}
	}

	return &command{
		defs:           defs,
		adminChannelID: adminChannelID,
	}
}

func (c *command) ClassicAliases() []string {
	return []string{"slash"}
}

func (c *command) SlashDef() *discord.SlashCommandCreate {
	return nil
}

func (c *command) Handle(req commands.Request) {
	if !commands.RequireAdminChannel(req, c.adminChannelID) {
		return
	}

	var operationType, targetGuildIDStr string
	req.ExtractArgs(commands.ArgsExtractor{
		Classic: func(ev *events.MessageCreate, argsLine string) {
			operationType, targetGuildIDStr, _ = strings.Cut(argsLine, " ")
		},
	})

	if operationType == operationTypePrint {
		buf, err := json.MarshalIndent(c.defs, "", "  ")
		if err != nil {
			req.Reply(commands.ReplyUnexpectedError)
			return
		}

		req.Reply(commands.Reply{
			Content: fmt.Sprintf("```%s```", messages.EscapeMarkdown(string(buf))),
		})
		return
	}

	targetGuildID, _ := snowflake.Parse(targetGuildIDStr)
	if (operationType != operationTypeRegister && operationType != operationTypeUnregister && operationType != operationTypePrint) || (targetGuildIDStr != "" && targetGuildID == 0) {
		req.Reply(commands.Reply{
			Content: fmt.Sprintf("%s **Usage: <%s/%s/%s> <?guildID>**",
				messages.IconUserError,
				operationTypeRegister,
				operationTypeUnregister,
				operationTypePrint,
			),
		})
		return
	}

	var slashDefinitionsToSet []discord.ApplicationCommandCreate
	if operationType == operationTypeRegister {
		slashDefinitionsToSet = c.defs
	}

	if targetGuildID == 0 {
		if _, err := req.Client().Rest.SetGlobalCommands(req.Client().ApplicationID, slashDefinitionsToSet, rest.WithCtx(req.Ctx())); err != nil {
			req.Reply(commands.ReplyUnexpectedError)
			req.Logger().Error("Failed to set global slash commands", slog.Int("len", len(slashDefinitionsToSet)), slog.Any("err", err))
			return
		}

		req.Logger().Info("Updated global slash commands", slog.Int("len", len(slashDefinitionsToSet)))
		req.Reply(commands.Reply{
			Content: ":white_check_mark: **Updated slash commands for all guilds!**",
		})
	} else {
		if _, err := req.Client().Rest.SetGuildCommands(req.Client().ApplicationID, targetGuildID, slashDefinitionsToSet, rest.WithCtx(req.Ctx())); err != nil {
			req.Reply(commands.ReplyUnexpectedError)
			req.Logger().Error("Failed to set guild slash commands", slog.String("guildID", targetGuildID.String()), slog.Int("len", len(slashDefinitionsToSet)), slog.Any("err", err))
			return
		}

		var guildNameForMsg string
		if guild, ok := req.Client().Caches.Guild(targetGuildID); ok {
			guildNameForMsg = guild.Name
		} else {
			guildNameForMsg = targetGuildID.String()
		}

		req.Logger().Info("Updated guild slash commands", slog.String("guildID", targetGuildID.String()), slog.Int("len", len(slashDefinitionsToSet)))
		req.Reply(commands.Reply{
			Content: fmt.Sprintf(":white_check_mark: **Updated slash commands for guild %s!**", guildNameForMsg),
		})
	}

}
