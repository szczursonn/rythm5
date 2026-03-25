package musicbot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/audio/audiodisgo"
	"github.com/szczursonn/rythm5/internal/audio/audiotranscoderbuf"
	"github.com/szczursonn/rythm5/internal/media"
)

type session struct {
	bot                      *Bot
	logger                   *slog.Logger
	guildID                  snowflake.ID
	voiceConn                voice.Conn
	voiceConnOpen            bool
	destroyReason            sessionDestroyReason
	destroyReqCh             chan sessionDestroyReason
	destroyDoneCh            chan struct{}
	frameProvider            *audiodisgo.FrameProvider
	trackStreamSetupResultCh chan sessionTrackStreamSetupResult
	trackStreamSetupCancelFn context.CancelFunc
	trackEndedCh             chan error
	trackEnqueueCh           chan []media.Track
	trackSkipCh              chan struct{}
	inactivityTimer          *time.Timer

	mu            sync.Mutex
	ctx           context.Context
	cancelCtx     context.CancelFunc
	textChannelID snowflake.ID
	queue         []media.Track
	currentTrack  media.Track
	looping       bool
}

type sessionDestroyReason string

const (
	sessionDestroyReasonShutdown          sessionDestroyReason = "shutdown"
	sessionDestroyReasonOpenFailed        sessionDestroyReason = "open failed"
	sessionDestroyReasonInactivityTimeout sessionDestroyReason = "inactivity timeout"
	sessionDestroyReasonRequested         sessionDestroyReason = "requested"
	sessionDestroyReasonVoiceDisconnected sessionDestroyReason = "voice disconnected"
)

type sessionTrackStreamSetupResult struct {
	src *audiotranscoderbuf.BufferedTranscoder
	err error
}

func newSession(bot *Bot, guildID snowflake.ID, textChannelID snowflake.ID, voiceChannelID snowflake.ID) *session {
	s := &session{
		bot:                      bot,
		logger:                   bot.logger.With(slog.String("guildID", guildID.String())),
		guildID:                  guildID,
		voiceConn:                bot.client.VoiceManager.CreateConn(guildID),
		destroyReason:            sessionDestroyReasonShutdown,
		destroyReqCh:             make(chan sessionDestroyReason, 1),
		destroyDoneCh:            make(chan struct{}),
		trackStreamSetupResultCh: make(chan sessionTrackStreamSetupResult, 1),
		trackEndedCh:             make(chan error, 1),
		trackEnqueueCh:           make(chan []media.Track),
		trackSkipCh:              make(chan struct{}),
		textChannelID:            textChannelID,
		inactivityTimer:          time.NewTimer(bot.inactivityTimeout),
	}
	s.frameProvider = audiodisgo.NewFrameProvider(s.trackEndedCh)
	s.ctx, s.cancelCtx = context.WithCancel(bot.ctx)
	s.logger.Info("Session created")
	go s.worker(voiceChannelID)
	return s
}

func (s *session) Enqueue(tracks ...media.Track) {
	select {
	case s.trackEnqueueCh <- tracks:
	case <-s.ctx.Done():
	}
}

func (s *session) CurrentTrack() media.Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentTrack
}

func (s *session) Queue() []media.Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]media.Track, len(s.queue))
	copy(cp, s.queue)
	return cp
}

func (s *session) ClearQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = nil
}

func (s *session) ShuffleQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.queue) - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		s.queue[i], s.queue[j] = s.queue[j], s.queue[i]
	}
}

func (s *session) Skip() {
	select {
	case s.trackSkipCh <- struct{}{}:
	case <-s.ctx.Done():
	}
}

func (s *session) SetLooping(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.looping = on
}

func (s *session) Looping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.looping
}

func (s *session) RequestDestroy(reason sessionDestroyReason) {
	select {
	case s.destroyReqCh <- reason:
	default:
	}
}

func (s *session) DestroyDone() <-chan struct{} {
	return s.destroyDoneCh
}

func (s *session) worker(initialVoiceChannelID snowflake.ID) {
	connOpenResultCh := make(chan error, 1)

	defer func() {
		s.cancelCtx()

		s.frameProvider.Close()
		if connOpenResultCh == nil {
			if s.voiceConnOpen {
				s.voiceConn.Close(s.bot.shutdownCtx)
			}
		} else {
			select {
			case err := <-connOpenResultCh:
				if err != nil {
					s.voiceConn.Close(s.bot.shutdownCtx)
				}
			case <-s.bot.shutdownCtx.Done():
			}
		}

		s.bot.guildIdToSessionMu.Lock()
		delete(s.bot.guildIdToSession, s.guildID)
		s.bot.guildIdToSessionMu.Unlock()

		s.logger.Info("Session destroyed", slog.String("reason", string(s.destroyReason)))
		close(s.destroyDoneCh)
	}()

	go func() {
		connOpenCtx, connOpenCtxCancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer connOpenCtxCancel()
		connOpenResultCh <- s.voiceConn.Open(connOpenCtx, initialVoiceChannelID, false, true)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case reason := <-s.destroyReqCh:
			s.workerHandleDestroyRequest(reason)
		case <-s.inactivityTimer.C:
			s.workerHandleInacitivityTimeout()
		case err := <-connOpenResultCh:
			connOpenResultCh = nil
			s.workerHandleConnOpenResult(err)
		case result := <-s.trackStreamSetupResultCh:
			s.workerHandleTrackStreamSetupResult(result)
		case err := <-s.trackEndedCh:
			s.workerHandleTrackEnded(err)
		case tracks := <-s.trackEnqueueCh:
			s.workerHandleEnqueue(tracks)
		case <-s.trackSkipCh:
			s.workerHandleSkip()
		}
	}
}

func (s *session) workerHandleDestroyRequest(reason sessionDestroyReason) {
	s.destroyReason = reason
	s.cancelCtx()
}

func (s *session) workerHandleInacitivityTimeout() {
	s.destroyReason = sessionDestroyReasonInactivityTimeout
	s.cancelCtx()
}

func (s *session) workerHandleConnOpenResult(err error) {
	if err != nil {
		s.cancelCtx()
		s.destroyReason = sessionDestroyReasonOpenFailed
		s.logger.Error("Failed to open voice connection", slog.Any("err", err))
		return
	}
	s.voiceConn.SetOpusFrameProvider(s.frameProvider)

	s.voiceConnOpen = true
	s.logger.Debug("Opened voice connection")
	s.advanceQueue()
}

func (s *session) workerHandleTrackStreamSetupResult(result sessionTrackStreamSetupResult) {
	s.trackStreamSetupCancelFn()
	s.trackStreamSetupCancelFn = nil

	if result.err != nil {
		go s.bot.client.Rest.CreateMessage(s.textChannelID, discord.MessageCreate{
			Content: iconAppError + " **An unexpected error has occured when starting playback**",
			Flags:   discord.MessageFlagSuppressNotifications,
		}, rest.WithCtx(s.ctx))
		s.logger.Error("Failed to set up track stream", slog.String("trackTitle", s.currentTrack.GetTitle()), slog.Any("err", result.err))
		s.advanceQueue()
		return
	}

	s.frameProvider.SetSource(result.src)
	s.logger.Debug("Track stream setup done", slog.String("trackTitle", s.currentTrack.GetTitle()), slog.Any("err", result.err))
}

func (s *session) workerHandleTrackEnded(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		go s.bot.client.Rest.CreateMessage(s.textChannelID, discord.MessageCreate{
			Content: iconAppError + " **An unexpected error has occured during playback**",
			Flags:   discord.MessageFlagSuppressNotifications,
		}, rest.WithCtx(s.ctx))
		s.logger.Error("Track ended with error", slog.String("trackTitle", s.currentTrack.GetTitle()), slog.Any("err", err))
	} else {
		s.logger.Debug("Track ended", slog.String("trackTitle", s.currentTrack.GetTitle()), slog.Any("err", err))
	}

	s.advanceQueue()
}

func (s *session) workerHandleEnqueue(tracks []media.Track) {
	s.mu.Lock()
	s.queue = append(s.queue, tracks...)
	s.mu.Unlock()
	if s.currentTrack == nil {
		s.advanceQueue()
	}
}

func (s *session) workerHandleSkip() {
	s.advanceQueue()
}

func (s *session) advanceQueue() {
	if !s.voiceConnOpen {
		return
	}

	if s.trackStreamSetupCancelFn != nil {
		s.trackStreamSetupCancelFn()
		// handler will call advanceQueue
		return
	}

	s.frameProvider.SetSource(nil)

	s.mu.Lock()
	if !s.looping {
		if len(s.queue) > 0 {
			s.currentTrack = s.queue[0]
			s.queue = s.queue[1:]
		} else {
			s.currentTrack = nil
		}
	}
	s.mu.Unlock()

	if s.currentTrack == nil {
		s.inactivityTimer.Reset(s.bot.inactivityTimeout)
		s.logger.Debug("Session inactivity timer reset")
		return
	}

	if !s.inactivityTimer.Stop() {
		select {
		case <-s.inactivityTimer.C:
		default:
		}
		s.logger.Debug("Session inactivity timer stopped")
	}

	s.logger.Debug("Setting up track stream", slog.String("trackTitle", s.currentTrack.GetTitle()))

	var trackStreamSetupCtx context.Context
	trackStreamSetupCtx, s.trackStreamSetupCancelFn = context.WithTimeout(s.ctx, 15*time.Second)

	go func() {
		s.trackStreamSetupResultCh <- func() sessionTrackStreamSetupResult {
			stream, err := s.currentTrack.GetStream(trackStreamSetupCtx)
			if err != nil {
				return sessionTrackStreamSetupResult{err: err}
			}

			transcoder, err := audiotranscoderbuf.NewBufferedTranscoder(stream, s.bot.transcoderOptions)
			if err != nil {
				stream.Close()
				return sessionTrackStreamSetupResult{err: err}
			}

			return sessionTrackStreamSetupResult{src: transcoder}
		}()
	}()
}
