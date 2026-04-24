package healthcheck

import (
	"context"
	"log/slog"
	"strings"

	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
)

const errPrefix = "musicbot/healthcheck: "

type RunnerOptions struct {
	Logger        *slog.Logger
	QueryResolver *media.QueryResolver
	Checks        []Check
}

func (opts *RunnerOptions) validateAndApplyDefaults() {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	if opts.QueryResolver == nil {
		panic(errPrefix + "query resolver must not be nil")
	}
}

type Check struct {
	Label string
	Query string
}

type Failure struct {
	Check *Check
	Err   error
}

type Runner struct {
	logger        *slog.Logger
	queryResolver *media.QueryResolver
	checks        []Check
}

func NewRunner(opts RunnerOptions) *Runner {
	opts.validateAndApplyDefaults()

	return &Runner{
		logger:        opts.Logger,
		queryResolver: opts.QueryResolver,
		checks:        opts.Checks,
	}
}

func (r *Runner) Run(ctx context.Context) []Failure {
	r.logger.Debug("Running health checks")

	failures := make([]Failure, 0, len(r.checks))
	for _, check := range r.checks {
		_, err := r.queryResolver.Query(ctx, check.Query)

		if err != nil {
			r.logger.Error("Health check failure", slog.String("label", check.Label), slog.String("query", check.Query), slog.Any("err", err))
			failures = append(failures, Failure{
				Check: &check,
				Err:   err,
			})
		} else {
			r.logger.Debug("Health check success", slog.String("label", check.Label), slog.String("query", check.Query))
		}
	}

	r.logger.Debug("Health checks done")

	return failures
}

func MakeFailureMessage(failures []Failure) string {
	var sb strings.Builder

	sb.WriteString(messages.IconAppError + " **Health check failed**")
	for _, result := range failures {
		sb.WriteString("\n- **")
		sb.WriteString(messages.EscapeMarkdown(result.Check.Label))
		sb.WriteString("** (")
		sb.WriteString(messages.EscapeMarkdown(result.Check.Query))
		sb.WriteString(")\n```")
		sb.WriteString(messages.EscapeMarkdown(result.Err.Error()))
		sb.WriteString("```")
	}

	return sb.String()
}
