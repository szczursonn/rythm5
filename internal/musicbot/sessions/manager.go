package sessions

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/transcode"
)

const errPrefix = "musicbot/sessions: "

var ErrSessionLimitHit = errors.New(errPrefix + "too many concurrent sessions")

type Options struct {
	Logger            *slog.Logger
	Client            *bot.Client
	InactivityTimeout time.Duration
	TranscoderOptions transcode.Options
	MaxSessions       int
}

func (opts *Options) validateAndApplyDefaults() {
	if opts.Client == nil {
		panic(errPrefix + "client must not be nil")
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	if opts.InactivityTimeout <= 0 {
		opts.InactivityTimeout = 5 * time.Minute
	}

	if opts.MaxSessions <= 0 {
		opts.MaxSessions = -1
	}
}

type Manager struct {
	logger            *slog.Logger
	client            *bot.Client
	inactivityTimeout time.Duration
	transcoderOptions transcode.Options
	maxSessions       int
	eventListener     bot.EventListener

	mu       sync.Mutex
	sessions map[snowflake.ID]*Session
}

func NewManager(opts Options) *Manager {
	opts.validateAndApplyDefaults()

	m := &Manager{
		logger:            opts.Logger,
		client:            opts.Client,
		inactivityTimeout: opts.InactivityTimeout,
		maxSessions:       opts.MaxSessions,
		transcoderOptions: opts.TranscoderOptions,
		sessions:          make(map[snowflake.ID]*Session),
	}
	m.eventListener = bot.NewListenerFunc(m.handleGuildVoiceLeave)

	m.client.AddEventListeners(m.eventListener)

	return m
}

func (m *Manager) Get(guildID snowflake.ID) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[guildID]
}

func (m *Manager) GetOrCreate(guildID, textChannelID, voiceChannelID snowflake.ID) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[guildID]; ok {
		return s, nil
	}

	if m.maxSessions != -1 && len(m.sessions)+1 > m.maxSessions {
		return nil, ErrSessionLimitHit
	}

	s := newSession(m, guildID, textChannelID, voiceChannelID)
	m.sessions[guildID] = s
	return s, nil
}

func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.list()
}

func (m *Manager) list() []*Session {
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}

func (m *Manager) detach(guildID snowflake.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, guildID)
}

func (m *Manager) handleGuildVoiceLeave(ev *events.GuildVoiceLeave) {
	if ev.VoiceState.UserID != ev.Client().ID() {
		return
	}

	s := m.Get(ev.VoiceState.GuildID)
	if s == nil {
		return
	}

	s.handleVoiceLeave()
}

func (m *Manager) Destroy(destroyCtx context.Context) {
	m.client.RemoveEventListeners(m.eventListener)

	m.mu.Lock()
	m.maxSessions = 0
	sessions := m.list()
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Go(func() {
			s.destroy(destroyCtx, DestroyReasonManagerDestroy)
		})
	}
	wg.Wait()
}
