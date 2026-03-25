package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Debug               bool          `toml:"debug"`
	LogPath             string        `toml:"log_path"`
	DiscordToken        string        `toml:"discord_token"`
	CommandPrefix       string        `toml:"command_prefix"`
	AdminChannelID      string        `toml:"admin_channel_id"`
	InactivityTimeout   time.Duration `toml:"inactivity_timeout"`
	Encoder             Encoder       `toml:"encoder"`
	YtDlp               YtDlp         `toml:"ytdlp"`
	HealthCheckInterval time.Duration `toml:"health_check_interval"`
	HealthChecks        []HealthCheck `toml:"health_check"`
}

type YtDlp struct {
	Path           string `toml:"path"`
	CookiePath     string `toml:"cookie_path"`
	CacheEnabled   bool   `toml:"cache_enabled"`
	CacheDir       string `toml:"cache_dir"`
	MaxConcurrency int    `toml:"max_concurrency"`
}

type Encoder struct {
	FfmpegPath     string        `toml:"ffmpeg_path"`
	Bitrate        int           `toml:"bitrate"`
	BufferDuration time.Duration `toml:"buffer_duration"`
}

type HealthCheck struct {
	Label string `toml:"label"`
	Query string `toml:"query"`
}

func LoadTOML() (*Config, error) {
	cfg := &Config{}

	if _, err := toml.DecodeFile("rythm5.toml", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
