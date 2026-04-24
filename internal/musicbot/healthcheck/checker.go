package healthcheck

import (
	"context"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

type CheckerOptions struct {
	Runner                 *Runner
	Client                 *bot.Client
	NotificationsChannelID *snowflake.ID
	Interval               time.Duration
}

func (opts *CheckerOptions) validateAndApplyDefaults() {
	if opts.Runner == nil {
		panic(errPrefix + "runner must not be nil")
	}

	if opts.Client == nil {
		panic(errPrefix + "client must not be nil")
	}

	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Hour
	}
}

type Checker struct {
	runner                 *Runner
	client                 *bot.Client
	interval               time.Duration
	notificationsChannelID *snowflake.ID

	ctx          context.Context
	cancelCtx    context.CancelFunc
	workerDoneCh chan struct{}
}

func NewChecker(opts CheckerOptions) *Checker {
	opts.validateAndApplyDefaults()

	c := &Checker{
		runner:                 opts.Runner,
		client:                 opts.Client,
		interval:               opts.Interval,
		notificationsChannelID: opts.NotificationsChannelID,
		workerDoneCh:           make(chan struct{}),
	}
	c.ctx, c.cancelCtx = context.WithCancel(context.Background())

	go c.worker()

	return c
}

func (c *Checker) worker() {
	defer close(c.workerDoneCh)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
		}

		if failures := c.runner.Run(c.ctx); c.notificationsChannelID != nil && len(failures) > 0 {
			if _, err := c.client.Rest.CreateMessage(*c.notificationsChannelID, discord.MessageCreate{
				Content: MakeFailureMessage(failures),
				Flags:   discord.MessageFlagSuppressEmbeds,
			}, rest.WithCtx(c.ctx)); err != nil {
				c.runner.logger.Error("Failed to send health check failure message", slog.Any("err", err))
			}
		}
	}
}
func (c *Checker) Stop(ctx context.Context) {
	c.cancelCtx()
	select {
	case <-c.workerDoneCh:
	case <-ctx.Done():
	}
}
