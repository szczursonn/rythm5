package musicbot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave/golibdave"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/audio/audiotranscoderbuf"
	"github.com/szczursonn/rythm5/internal/logging/loglevellimit"
	"github.com/szczursonn/rythm5/internal/media"
)

type Bot struct {
	logger               *slog.Logger
	client               *bot.Client
	mediaProvider        media.TrackProvider
	inactivityTimeout    time.Duration
	transcoderOptions    audiotranscoderbuf.BufferedTranscoderOptions
	classicCommandPrefix string
	adminChannelID       *snowflake.ID
	healthChecks         []HealthCheck
	healthCheckInterval  time.Duration

	guildIdToSession   map[snowflake.ID]*session
	guildIdToSessionMu sync.Mutex

	ctx               context.Context
	cancelCtx         context.CancelFunc
	cmdHandlersWg     sync.WaitGroup
	shutdownCtx       context.Context
	shutdownCancelCtx context.CancelFunc
}

type MusicBotOptions struct {
	Logger               *slog.Logger
	DiscordToken         string
	ClassicCommandPrefix string
	InactivityTimeout    time.Duration
	MediaProvider        media.TrackProvider
	TranscoderOptions    audiotranscoderbuf.BufferedTranscoderOptions
	AdminChannelID       *snowflake.ID
	HealthChecks         []HealthCheck
	HealthCheckInterval  time.Duration
}

func Start(ctx context.Context, options MusicBotOptions) (*Bot, error) {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.InactivityTimeout <= 0 {
		options.InactivityTimeout = 5 * time.Minute
	}
	if options.ClassicCommandPrefix == "" {
		options.ClassicCommandPrefix = "!"
	}
	if options.HealthCheckInterval <= 0 {
		options.HealthCheckInterval = 5 * time.Hour
	}

	if options.DiscordToken == "" {
		return nil, fmt.Errorf("musicbot: missing discord token")
	}
	if options.MediaProvider == nil {
		return nil, fmt.Errorf("musicbot: missing media provider")
	}

	b := &Bot{
		logger:               options.Logger,
		mediaProvider:        options.MediaProvider,
		inactivityTimeout:    options.InactivityTimeout,
		transcoderOptions:    options.TranscoderOptions,
		guildIdToSession:     make(map[snowflake.ID]*session),
		classicCommandPrefix: options.ClassicCommandPrefix,
		adminChannelID:       options.AdminChannelID,
		healthChecks:         options.HealthChecks,
		healthCheckInterval:  options.HealthCheckInterval,
	}
	b.ctx, b.cancelCtx = context.WithCancel(context.Background())
	b.shutdownCtx, b.shutdownCancelCtx = context.WithCancel(context.Background())

	client, err := disgo.New(options.DiscordToken,
		bot.WithLogger(
			slog.New(loglevellimit.NewLevelLimitHandler(b.logger.Handler(), slog.LevelWarn)).With(slog.String("module", "disgo")),
		),
		bot.WithDefaultGateway(),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
				gateway.IntentMessageContent,
				gateway.IntentDirectMessages,
				gateway.IntentGuildVoiceStates,
			),
			gateway.WithAutoReconnect(true),
		),
		bot.WithVoiceManagerConfigOpts(
			voice.WithDaveSessionCreateFunc(golibdave.NewSession),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnMessageCreate:                 b.handleMessageCreateEvent,
			OnApplicationCommandInteraction: b.handleApplicationCommandInteractionCreateEvent,
			OnReady:                         b.handleReadyEvent,
			OnGuildVoiceLeave:               b.handleGuildVoiceLeaveEvent,
		}),
	)
	if err != nil {
		b.cancelCtx()
		return nil, fmt.Errorf("musicbot: creating disgo client: %w", err)
	}
	b.client = client

	if err := b.client.OpenGateway(ctx); err != nil {
		b.cancelCtx()
		b.client.Close(ctx)
		return nil, fmt.Errorf("musicbot: opening gateway: %w", err)
	}

	go b.healthCheckWorker()

	return b, nil
}

func (b *Bot) Stop(ctx context.Context) {
	b.cancelCtx()
	defer b.shutdownCancelCtx()
	go func() {
		select {
		case <-ctx.Done():
			b.shutdownCancelCtx()
		case <-b.shutdownCtx.Done():
		}
	}()

	b.cmdHandlersWg.Wait()
	b.guildIdToSessionMu.Lock()
	sessions := make([]*session, 0, len(b.guildIdToSession))
	for _, s := range b.guildIdToSession {
		sessions = append(sessions, s)
	}
	b.guildIdToSessionMu.Unlock()

	for _, s := range sessions {
		s.RequestDestroy(sessionDestroyReasonShutdown)
	}
	for _, s := range sessions {
		select {
		case <-s.DestroyDone():
		case <-b.shutdownCtx.Done():
		}
	}

	if b.client != nil {
		b.client.Close(b.shutdownCtx)
	}

	b.logger.Info("Stopped")
}

func (b *Bot) getSession(guildID snowflake.ID) *session {
	b.guildIdToSessionMu.Lock()
	defer b.guildIdToSessionMu.Unlock()
	return b.guildIdToSession[guildID]
}

func (b *Bot) getOrCreateSession(guildID, textChannelID, voiceChannelID snowflake.ID) *session {
	b.guildIdToSessionMu.Lock()
	defer b.guildIdToSessionMu.Unlock()

	if s, ok := b.guildIdToSession[guildID]; ok {
		return s
	}

	s := newSession(b, guildID, textChannelID, voiceChannelID)
	b.guildIdToSession[guildID] = s
	return s
}

func (b *Bot) handleReadyEvent(ev *events.Ready) {
	b.logger.Info("Logged into Discord", slog.String("userId", ev.User.ID.String()), slog.String("username", ev.User.Username))
}

func (b *Bot) handleGuildVoiceLeaveEvent(ev *events.GuildVoiceLeave) {
	if ev.VoiceState.UserID != ev.Client().ID() {
		return
	}

	s := b.getSession(ev.VoiceState.GuildID)
	if s == nil {
		return
	}

	s.RequestDestroy(sessionDestroyReasonVoiceDisconnected)
}
