package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave/golibdave"
	"github.com/szczursonn/rythm5/internal/config"
	"github.com/szczursonn/rythm5/internal/httpaudio"
	"github.com/szczursonn/rythm5/internal/logging/logfile"
	"github.com/szczursonn/rythm5/internal/logging/loglevellimit"
	"github.com/szczursonn/rythm5/internal/logging/logmulti"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/media/discordattachment"
	"github.com/szczursonn/rythm5/internal/media/spotify"
	"github.com/szczursonn/rythm5/internal/media/ytdlp"
	"github.com/szczursonn/rythm5/internal/musicbot/commands"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/clearcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/disconnectcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/healthcheckcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/loopcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/playcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/queuecmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/shufflecmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/skipcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/commands/slashcmd"
	"github.com/szczursonn/rythm5/internal/musicbot/healthcheck"
	"github.com/szczursonn/rythm5/internal/musicbot/sessions"
	"github.com/szczursonn/rythm5/internal/transcode"
)

func main() {
	cfgPath := flag.String("config", "rythm5.toml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)

	logLevel := slog.LevelInfo
	if cfg != nil && cfg.Logs.Debug {
		logLevel = slog.LevelDebug
	}

	logHandlers := make([]slog.Handler, 0, 2)
	logHandlers = append(logHandlers, slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	var logFileErr error
	if cfg != nil && cfg.Logs.File.Enabled {
		logFilePath := cfg.Logs.File.Path
		if logFilePath == "" {
			logFilePath = "rythm5.jsonl"
		}

		var logFile io.WriteCloser
		logFile, logFileErr = logfile.NewBufferedLogFile(logfile.Options{
			Path:          logFilePath,
			BufferSize:    cfg.Logs.File.BufferSize,
			FlushInterval: cfg.Logs.File.FlushInterval,
		})
		if logFileErr == nil {
			defer logFile.Close()
			logHandlers = append(logHandlers, slog.NewJSONHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			}))
		}
	}

	logger := slog.New(logmulti.NewMultiHandler(logHandlers...))
	slog.SetDefault(logger.With("module", "global"))

	if logFileErr != nil {
		logger.Error("Failed to initialize logfile", slog.Any("err", logFileErr))
	}

	if err != nil {
		logger.Error("Failed to load config", slog.Any("err", err))
		return
	}

	shutdownCtxControllerDoneCh := make(chan struct{})
	defer func() {
		<-shutdownCtxControllerDoneCh
	}()
	ctx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelCtx()
	shutdownCtx, cancelShutdownCtx := context.WithCancel(context.Background())
	defer cancelShutdownCtx()
	go func() {
		shutdownTimeout := cfg.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = 15 * time.Second
		}

		defer close(shutdownCtxControllerDoneCh)
		<-ctx.Done()
		// Allow next signal to instakill the app
		cancelCtx()

		select {
		case <-time.After(shutdownTimeout):
			cancelShutdownCtx()
		case <-shutdownCtx.Done():
		}
	}()

	httpAudio := httpaudio.NewClient(httpaudio.ClientOptions{})

	ytdlpQuerySource := ytdlp.NewQuerySource(ytdlp.QuerySourceOptions{
		HttpAudio:         httpAudio,
		BinaryPath:        cfg.YtDlp.Path,
		CookieFilePath:    cfg.YtDlp.CookiePath,
		CacheEnabled:      cfg.YtDlp.CacheEnabled,
		CacheDir:          cfg.YtDlp.CacheDir,
		MaxConcurrency:    cfg.YtDlp.MaxConcurrency,
		CPUPriority:       cfg.YtDlp.CPUPriority,
		OOMKillerPriority: cfg.YtDlp.OOMKillerPriority,
	})

	spotifyQuerySource := spotify.NewQueryHandler(spotify.QueryHandlerOptions{
		StreamableQuerySource: ytdlpQuerySource,
	})

	queryResolver := media.NewQueryResolver(ytdlpQuerySource, spotifyQuerySource)
	discordAttachmentSource := discordattachment.NewProvider(httpAudio)

	client, err := disgo.New(cfg.DiscordToken,
		bot.WithLogger(
			slog.New(loglevellimit.NewLevelLimitHandler(logger.Handler(), slog.LevelWarn)).With(slog.String("module", "disgo")),
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
		bot.WithEventListenerFunc(func(ev *events.Ready) {
			logger.Info("Logged into Discord", slog.String("userId", ev.User.ID.String()), slog.String("username", ev.User.Username))
		}),
	)
	if err != nil {
		logger.Error("Failed to create disgo client", slog.Any("err", err))
		return
	}
	defer func() {
		logger.Info("Shutting down disgo client...")
		client.Close(shutdownCtx)
		logger.Info("Shut down disgo client")
	}()

	sessionManager := sessions.NewManager(sessions.Options{
		Logger:            logger.With("module", "sessions"),
		Client:            client,
		InactivityTimeout: cfg.Sessions.InactivityTimeout,
		TranscoderOptions: transcode.Options{
			FfmpegPath:        cfg.Transcoder.FfmpegPath,
			Bitrate:           cfg.Transcoder.Bitrate,
			BufferDuration:    cfg.Transcoder.BufferDuration,
			CPUPriority:       cfg.Transcoder.CPUPriority,
			OOMKillerPriority: cfg.Transcoder.OOMKillerPriority,
		},
		MaxSessions: cfg.Sessions.Limit,
	})
	defer func() {
		logger.Info("Shutting down session manager...")
		sessionManager.Destroy(shutdownCtx)
		logger.Info("Shut down session manager")
	}()

	var healthCheckRunner *healthcheck.Runner
	if len(cfg.HealthCheck.Checks) > 0 {
		healthCheckRunner = healthcheck.NewRunner(healthcheck.RunnerOptions{
			Logger:        logger.With("module", "healthcheck"),
			QueryResolver: queryResolver,
			Checks:        cfg.HealthCheck.Checks,
		})

		healthCheckChecker := healthcheck.NewChecker(healthcheck.CheckerOptions{
			Runner:                 healthCheckRunner,
			Client:                 client,
			NotificationsChannelID: cfg.AdminChannelID,
			Interval:               cfg.HealthCheck.Interval,
		})
		defer func() {
			logger.Info("Shutting down health check checker...")
			healthCheckChecker.Stop(shutdownCtx)
			logger.Info("Shut down health check checker")
		}()
	}

	cmds := []commands.Command{
		clearcmd.New(sessionManager),
		disconnectcmd.New(sessionManager),
		loopcmd.New(sessionManager),
		playcmd.New(sessionManager, queryResolver, discordAttachmentSource),
		queuecmd.New(sessionManager),
		shufflecmd.New(sessionManager),
		skipcmd.New(sessionManager),
	}

	if cfg.AdminChannelID != nil {
		if healthCheckRunner != nil {
			cmds = append(cmds, healthcheckcmd.New(healthCheckRunner, *cfg.AdminChannelID))
		}
		cmds = append(cmds, slashcmd.New(cmds, *cfg.AdminChannelID))
	}

	commandDispatcher := commands.NewDispatcher(commands.DispatcherOptions{
		Logger:        logger.With("module", "commands"),
		Client:        client,
		ClassicPrefix: cfg.Commands.ClassicPrefix,
		Commands:      cmds,
	})
	defer func() {
		logger.Info("Shutting down command dispatcher...")
		commandDispatcher.Stop()
		logger.Info("Shut down command dispatcher")
	}()

	logger.Info("Connecting to Discord...")
	if err := client.OpenGateway(ctx); err != nil {
		logger.Error("Failed to connect to Discord gateway", slog.Any("err", err))
		return
	}

	<-ctx.Done()
}
