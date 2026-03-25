package musicbot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/mdutil"
)

func initSlashChatCommand(slashCommandDefinitions []discord.ApplicationCommandCreate) *chatCommand {
	const commandAlias = "slash"

	const (
		operationTypeRegister   = "register"
		operationTypeUnregister = "unregister"
		operationTypePrint      = "print"
	)

	return &chatCommand{
		ClassicMeta: &chatCommandClassicMetadata{
			Aliases: []string{commandAlias},
		},
		Handler: func(cctx chatCommandContext) {
			if !chatCommandRequireAdminChannel(cctx) {
				return
			}

			operationType, targetGuildIDStr, _ := strings.Cut(cctx.(*classicChatCommandContext).args, " ")

			if operationType == operationTypePrint {
				buf, err := json.MarshalIndent(slashCommandDefinitions, "", "  ")
				if err != nil {
					cctx.Reply(replyUnexpectedError)
					return
				}

				cctx.Reply(chatCommandReply{
					Content: fmt.Sprintf("```%s```", mdutil.EscapeMarkdown(string(buf))),
				})
				return
			}

			targetGuildID, _ := snowflake.Parse(targetGuildIDStr)
			if (operationType != operationTypeRegister && operationType != operationTypeUnregister && operationType != operationTypePrint) || (targetGuildIDStr != "" && targetGuildID == 0) {
				cctx.Reply(chatCommandReply{
					Content: fmt.Sprintf("%s **Usage: %s%s <%s/%s/%s> <?guildID>**",
						iconUserError,
						cctx.Bot().classicCommandPrefix,
						commandAlias,
						operationTypeRegister,
						operationTypeUnregister,
						operationTypePrint,
					),
					Ephemeral: true,
				})
				return
			}

			var slashDefinitionsToSet []discord.ApplicationCommandCreate
			if operationType == operationTypeRegister {
				slashDefinitionsToSet = slashCommandDefinitions
			}

			if targetGuildID == 0 {
				if _, err := cctx.Bot().client.Rest.SetGlobalCommands(cctx.Bot().client.ApplicationID, slashDefinitionsToSet, rest.WithCtx(cctx.Bot().ctx)); err != nil {
					cctx.Reply(replyUnexpectedError)
					cctx.Logger().Error("Failed to set global slash commands", slog.Int("len", len(slashDefinitionsToSet)), slog.Any("err", err))
					return
				}

				cctx.Logger().Info("Updated global slash commands", slog.Int("len", len(slashDefinitionsToSet)))

			} else {
				if _, err := cctx.Bot().client.Rest.SetGuildCommands(cctx.Bot().client.ApplicationID, targetGuildID, slashDefinitionsToSet, rest.WithCtx(cctx.Bot().ctx)); err != nil {
					cctx.Reply(replyUnexpectedError)
					cctx.Logger().Error("Failed to set guild slash commands", slog.String("guildID", targetGuildID.String()), slog.Int("len", len(slashDefinitionsToSet)), slog.Any("err", err))
					return
				}

				cctx.Logger().Info("Updated guild slash commands", slog.String("guildID", targetGuildID.String()), slog.Int("len", len(slashDefinitionsToSet)))
			}

			cctx.Reply(chatCommandReply{
				Content: ":white_check_mark: **Done!**",
			})
		},
	}
}
