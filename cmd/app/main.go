package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/audio/audiotranscoder"
	"github.com/szczursonn/rythm5/internal/audio/audiotranscoderbuf"
	"github.com/szczursonn/rythm5/internal/config"
	"github.com/szczursonn/rythm5/internal/logging/logfile"
	"github.com/szczursonn/rythm5/internal/logging/logmulti"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/media/mediaspotify"
	"github.com/szczursonn/rythm5/internal/media/mediaytdlp"
	"github.com/szczursonn/rythm5/internal/musicbot"
	"github.com/szczursonn/rythm5/internal/spotifyinfo"
	"github.com/szczursonn/rythm5/internal/ytdlp"
)

func initLogger(cfg *config.Config) (*slog.Logger, func()) {

	logLevel := slog.LevelInfo
	if cfg != nil && cfg.Debug {
		logLevel = slog.LevelDebug
	}

	handlers := make([]slog.Handler, 0, 2)
	cleanupFn := func() {}
	handlers = append(handlers, slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	var logFileErr error

	if cfg != nil {
		logFilePath := cfg.LogPath
		if logFilePath == "" {
			logFilePath = "rythm5.log"
		}

		var logFile io.WriteCloser
		logFile, logFileErr = logfile.NewBufferedLogFile(logFilePath, 8192, time.Second)
		if logFileErr == nil {
			handlers = append(handlers, slog.NewJSONHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			}))
			cleanupFn = func() {
				logFile.Close()
			}
		}
	}

	logger := slog.New(logmulti.NewMultiHandler(handlers...))

	if logFileErr != nil {
		logger.Error("Failed to initialize logfile", slog.Any("err", logFileErr))
	}

	slog.SetDefault(logger.With("module", "global"))
	return logger, cleanupFn
}

func initMediaProviders(cfg *config.Config) media.TrackProvider {
	ytdlpProvider := mediaytdlp.NewProvider(ytdlp.NewClient(ytdlp.ClientOptions{
		BinaryPath:     cfg.YtDlp.Path,
		CookieFilePath: cfg.YtDlp.CookiePath,
		CacheEnabled:   cfg.YtDlp.CacheEnabled,
		CacheDir:       cfg.YtDlp.CacheDir,
		MaxConcurrency: cfg.YtDlp.MaxConcurrency,
	}))

	return media.NewMultiProvider([]media.TrackProvider{
		ytdlpProvider,
		mediaspotify.NewProvider(&spotifyinfo.Client{}, ytdlpProvider),
	})
}

func main() {
	cfg, err := config.LoadTOML()
	logger, loggerCleanupFn := initLogger(cfg)
	defer loggerCleanupFn()
	if err != nil {
		logger.Error("Failed to load config", slog.Any("err", err))
		return
	}

	mediaProvider := initMediaProviders(cfg)

	ctx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelCtx()

	var adminChannelID *snowflake.ID
	if cfg.AdminChannelID != "" {
		if id, err := snowflake.Parse(cfg.AdminChannelID); err != nil {
			logger.Error("Failed to parse admin channel id", slog.Any("err", err))
		} else {
			adminChannelID = &id
		}
	}

	healthChecks := make([]musicbot.HealthCheck, 0, len(cfg.HealthChecks))
	for _, cfgHealthCheck := range cfg.HealthChecks {
		healthChecks = append(healthChecks, musicbot.HealthCheck{
			Label: cfgHealthCheck.Label,
			Query: cfgHealthCheck.Query,
		})
	}

	logger.Info("Starting...")
	bot, err := musicbot.Start(ctx, musicbot.MusicBotOptions{
		Logger:               logger,
		DiscordToken:         cfg.DiscordToken,
		ClassicCommandPrefix: cfg.CommandPrefix,
		InactivityTimeout:    cfg.InactivityTimeout,
		MediaProvider:        mediaProvider,
		TranscoderOptions: audiotranscoderbuf.BufferedTranscoderOptions{
			TranscoderOptions: audiotranscoder.TranscoderOptions{
				FfmpegPath: cfg.Encoder.FfmpegPath,
				Bitrate:    cfg.Encoder.Bitrate,
			},
			BufferDuration: cfg.Encoder.BufferDuration,
		},
		AdminChannelID:      adminChannelID,
		HealthChecks:        healthChecks,
		HealthCheckInterval: cfg.HealthCheckInterval,
	})
	if err != nil {
		logger.Error("Failed to start bot", slog.Any("err", err))
		return
	}

	<-ctx.Done()
	// next signal will instakill the app
	cancelCtx()
	logger.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bot.Stop(shutdownCtx)
}
