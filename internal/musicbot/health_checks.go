package musicbot

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/szczursonn/rythm5/internal/mdutil"
	"github.com/szczursonn/rythm5/internal/media"
)

type HealthCheck struct {
	Label string
	Query string
}

type healthCheckFailure struct {
	healthCheck *HealthCheck
	err         error
}

func (b *Bot) runHealthChecks() []healthCheckFailure {
	b.logger.Debug("Running health checks")
	failures := make([]healthCheckFailure, 0, len(b.healthChecks))

	for _, healthCheck := range b.healthChecks {
		asURL, err := url.ParseRequestURI(healthCheck.Query)
		if err == nil {
			_, err = b.mediaProvider.QueryByURL(b.ctx, asURL, media.URLQueryOptions{
				Preference: media.QueryPreferenceTrack,
			})
		} else {
			_, err = b.mediaProvider.QueryBySearch(b.ctx, healthCheck.Query, media.SearchQueryOptions{
				MaxResults: 1,
			})
		}

		if err != nil {
			failures = append(failures, healthCheckFailure{
				healthCheck: &healthCheck,
				err:         err,
			})
			b.logger.Error("Health check failure", slog.String("label", healthCheck.Label), slog.String("query", healthCheck.Query), slog.Any("err", err))
		} else {
			b.logger.Debug("Health check success", slog.String("label", healthCheck.Label), slog.String("query", healthCheck.Query))
		}
	}
	b.logger.Debug("Health checks done")

	return failures
}

func (b *Bot) healthCheckWorker() {
	ticker := time.NewTicker(b.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
		}

		if failures := b.runHealthChecks(); len(failures) > 0 && b.adminChannelID != nil {
			b.client.Rest.CreateMessage(*b.adminChannelID, discord.MessageCreate{
				Content: createHealthCheckFailureMessage(failures),
				Flags:   discord.MessageFlagSuppressNotifications | discord.MessageFlagSuppressEmbeds,
			}, rest.WithCtx(b.ctx))
		}

	}
}

func createHealthCheckFailureMessage(failures []healthCheckFailure) string {
	var sb strings.Builder

	sb.WriteString(iconAppError)
	sb.WriteString(" **Health check failed** ")
	for _, failure := range failures {
		sb.WriteString("\n- **")
		sb.WriteString(mdutil.EscapeMarkdown(failure.healthCheck.Label))
		sb.WriteString("** (")
		sb.WriteString(mdutil.EscapeMarkdown(failure.healthCheck.Query))
		sb.WriteString(")\n")
		sb.WriteString(mdutil.EscapeMarkdown(failure.err.Error()))
	}

	return sb.String()
}
