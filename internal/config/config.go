package config

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/musicbot/healthcheck"
)

type rawConfig struct {
	DiscordToken   string `toml:"discord_token"`
	AdminChannelID string `toml:"admin_channel_id"`
	Logs           struct {
		Debug bool `toml:"debug"`
		File  struct {
			Enabled       bool          `toml:"enabled"`
			Path          string        `toml:"path"`
			FlushInterval time.Duration `toml:"flush_interval"`
			BufferSize    int           `toml:"buffer_size"`
		} `toml:"file"`
	} `toml:"logs"`
	Commands struct {
		ClassicPrefix string `toml:"classic_prefix"`
	} `toml:"commands"`
	Sessions struct {
		Limit             int           `toml:"limit"`
		InactivityTimeout time.Duration `toml:"inactivity_timeout"`
		TrackSetupTimeout time.Duration `toml:"track_setup_timeout"`
	} `toml:"sessions"`
	Transcoder struct {
		FfmpegPath     string        `toml:"ffmpeg_path"`
		Bitrate        int           `toml:"bitrate"`
		BufferDuration time.Duration `toml:"buffer_duration"`
		NiceValue      *int          `toml:"nice_value"`
		OOMScoreAdj    *int          `toml:"oom_score_adj"`
	} `toml:"transcoder"`
	YtDlp struct {
		Path           string `toml:"path"`
		CookiePath     string `toml:"cookie_path"`
		CacheEnabled   bool   `toml:"cache_enabled"`
		CacheDir       string `toml:"cache_dir"`
		MaxConcurrency int    `toml:"max_concurrency"`
		NiceValue      *int   `toml:"nice_value"`
		OOMScoreAdj    *int   `toml:"oom_score_adj"`
	} `toml:"ytdlp"`
	HealthCheck struct {
		Interval time.Duration `toml:"interval"`
		Checks   []struct {
			Label string `toml:"label"`
			Query string `toml:"query"`
		} `toml:"checks"`
	} `toml:"health_check"`
}

type Config struct {
	DiscordToken   string
	AdminChannelID *snowflake.ID
	Logs           Logs
	Commands       Commands
	Sessions       Sessions
	Transcoder     Transcoder
	YtDlp          YtDlp
	HealthCheck    HealthCheck
}

type Logs struct {
	Debug bool
	File  LogFile
}

type LogFile struct {
	Enabled       bool
	Path          string
	FlushInterval time.Duration
	BufferSize    int
}

type Commands struct {
	ClassicPrefix string
}

type Sessions struct {
	Limit             int
	InactivityTimeout time.Duration
	TrackSetupTimeout time.Duration
}

type Transcoder struct {
	FfmpegPath     string
	Bitrate        int
	BufferDuration time.Duration
	NiceValue      *int
	OOMScoreAdj    *int
}

type HealthCheck struct {
	Interval time.Duration
	Checks   []healthcheck.Check
}

type YtDlp struct {
	Path           string
	CookiePath     string
	CacheEnabled   bool
	CacheDir       string
	MaxConcurrency int
	NiceValue      *int
	OOMScoreAdj    *int
}

func Load(path string) (*Config, error) {
	rawCfg := rawConfig{}
	if _, err := toml.DecodeFile(path, &rawCfg); err != nil {
		return nil, err
	}

	var adminChannelID *snowflake.ID
	if rawCfg.AdminChannelID != "" {
		if id, err := snowflake.Parse(rawCfg.AdminChannelID); err == nil {
			adminChannelID = &id
		}
	}

	return &Config{
		DiscordToken:   rawCfg.DiscordToken,
		AdminChannelID: adminChannelID,
		Logs: Logs{
			Debug: rawCfg.Logs.Debug,
			File: LogFile{
				Enabled:       rawCfg.Logs.File.Enabled,
				Path:          rawCfg.Logs.File.Path,
				FlushInterval: rawCfg.Logs.File.FlushInterval,
				BufferSize:    rawCfg.Logs.File.BufferSize,
			},
		},
		Commands: Commands{
			ClassicPrefix: rawCfg.Commands.ClassicPrefix,
		},
		Sessions: Sessions{
			Limit:             rawCfg.Sessions.Limit,
			InactivityTimeout: rawCfg.Sessions.InactivityTimeout,
			TrackSetupTimeout: rawCfg.Sessions.TrackSetupTimeout,
		},
		Transcoder: Transcoder{
			FfmpegPath:     rawCfg.Transcoder.FfmpegPath,
			Bitrate:        rawCfg.Transcoder.Bitrate,
			BufferDuration: rawCfg.Transcoder.BufferDuration,
			NiceValue:      rawCfg.Transcoder.NiceValue,
			OOMScoreAdj:    rawCfg.Transcoder.OOMScoreAdj,
		},
		HealthCheck: HealthCheck{
			Interval: rawCfg.HealthCheck.Interval,
			Checks: func() []healthcheck.Check {
				arr := make([]healthcheck.Check, 0, len(rawCfg.HealthCheck.Checks))
				for _, c := range rawCfg.HealthCheck.Checks {
					arr = append(arr, healthcheck.Check{
						Label: c.Label,
						Query: c.Query,
					})
				}
				return arr
			}(),
		},
		YtDlp: YtDlp{
			Path:           rawCfg.YtDlp.Path,
			CookiePath:     rawCfg.YtDlp.CookiePath,
			CacheEnabled:   rawCfg.YtDlp.CacheEnabled,
			CacheDir:       rawCfg.YtDlp.CacheDir,
			MaxConcurrency: rawCfg.YtDlp.MaxConcurrency,
			NiceValue:      rawCfg.YtDlp.NiceValue,
			OOMScoreAdj:    rawCfg.YtDlp.OOMScoreAdj,
		},
	}, nil
}
