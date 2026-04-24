package sessions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
	"github.com/szczursonn/rythm5/internal/transcode"
	"github.com/szczursonn/rythm5/internal/transcode/disgoframe"
)

var ErrSessionDestroyed = errors.New(errPrefix + "session is destroyed")

type DestroyReason string

const (
	DestroyReasonOpenFailed        DestroyReason = "open failed"
	DestroyReasonInactivityTimeout DestroyReason = "inactivity timeout"
	DestroyReasonRequested         DestroyReason = "requested"
	DestroyReasonManagerDestroy    DestroyReason = "manager destroyed"
	DestroyReasonVoiceDisconnected DestroyReason = "voice disconnected"
)

type Session struct {
	// read-only
	manager                  *Manager
	logger                   *slog.Logger
	guildID                  snowflake.ID
	textChannelID            snowflake.ID
	voiceConn                voice.Conn
	frameProvider            *disgoframe.FrameProvider
	ctx                      context.Context
	cancelCtx                context.CancelFunc
	destroyCtx               context.Context
	cancelDestroyCtx         context.CancelFunc
	destroyReqCh             chan DestroyReason
	destroyDoneCh            chan struct{}
	trackEnqueueCh           chan trackEnqueueTask
	trackSkipCh              chan trackSkipTask
	trackStreamSetupResultCh chan trackStreamSetupResult

	// worker-only
	trackStreamSetupCancelFn context.CancelFunc
	voiceConnOpen            bool
	inactivityTimer          *time.Timer
	destroyReason            DestroyReason

	// mutex-protected
	mu           sync.Mutex
	queue        []media.Track
	currentTrack media.Track
	looping      bool
}

type trackEnqueueTask struct {
	tracks []media.Track
	doneCh chan bool
}

type trackSkipTask struct {
	doneCh chan bool
}

type trackStreamSetupResult struct {
	src *transcode.Transcoder
	err error
}

func newSession(m *Manager, guildID, textChannelID, voiceChannelID snowflake.ID) *Session {
	s := &Session{
		manager:                  m,
		logger:                   m.logger.With(slog.String("guildID", guildID.String())),
		guildID:                  guildID,
		textChannelID:            textChannelID,
		voiceConn:                m.client.VoiceManager.CreateConn(guildID),
		frameProvider:            disgoframe.NewFrameProvider(),
		destroyReqCh:             make(chan DestroyReason, 1),
		destroyDoneCh:            make(chan struct{}),
		trackEnqueueCh:           make(chan trackEnqueueTask),
		trackSkipCh:              make(chan trackSkipTask),
		trackStreamSetupResultCh: make(chan trackStreamSetupResult, 1),

		inactivityTimer: time.NewTimer(m.inactivityTimeout),
	}
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	s.destroyCtx, s.cancelDestroyCtx = context.WithCancel(context.Background())
	s.logger.Info("Session created")
	go s.worker(voiceChannelID)
	return s
}

func (s *Session) CurrentTrack() media.Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentTrack
}

func (s *Session) Queue() []media.Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]media.Track, len(s.queue))
	copy(cp, s.queue)
	return cp
}

func (s *Session) Enqueue(ctx context.Context, tracks ...media.Track) (bool, error) {
	doneCh := make(chan bool)

	select {
	case s.trackEnqueueCh <- trackEnqueueTask{
		tracks: tracks,
		doneCh: doneCh,
	}:
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.ctx.Done():
		return false, ErrSessionDestroyed
	}

	select {
	case isImmediatePlayback := <-doneCh:
		return isImmediatePlayback, nil
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.ctx.Done():
		return false, ErrSessionDestroyed
	}
}

func (s *Session) ClearQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.queue)
	s.queue = s.queue[:0]
}

func (s *Session) ShuffleQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.queue) - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		s.queue[i], s.queue[j] = s.queue[j], s.queue[i]
	}
}

func (s *Session) Skip(ctx context.Context) (bool, error) {
	doneCh := make(chan bool)

	select {
	case s.trackSkipCh <- trackSkipTask{
		doneCh: doneCh,
	}:
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.ctx.Done():
		return false, ErrSessionDestroyed
	}

	select {
	case hasSkipped := <-doneCh:
		return hasSkipped, nil
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.ctx.Done():
		return false, ErrSessionDestroyed
	}
}

func (s *Session) Looping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.looping
}

func (s *Session) SetLooping(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.looping = on
}

func (s *Session) Destroy(destroyCtx context.Context) {
	s.destroy(destroyCtx, DestroyReasonRequested)
}

func (s *Session) destroy(destroyCtx context.Context, reason DestroyReason) {
	select {
	case s.destroyReqCh <- reason:
	default:
	}

	select {
	case <-destroyCtx.Done():
		s.cancelDestroyCtx()
		<-s.destroyDoneCh
	case <-s.destroyDoneCh:
	}
}

func (s *Session) handleVoiceLeave() {
	select {
	case s.destroyReqCh <- DestroyReasonVoiceDisconnected:
	default:
	}
}

func (s *Session) worker(initialVoiceChannelID snowflake.ID) {
	connOpenResultCh := make(chan error, 1)

	defer func() {
		s.cancelCtx()
		if s.trackStreamSetupCancelFn != nil {
			s.trackStreamSetupCancelFn()
		}

		s.frameProvider.Close()
		if connOpenResultCh == nil {
			if s.voiceConnOpen {
				s.voiceConn.Close(s.destroyCtx)
			}
		} else {
			if err := <-connOpenResultCh; err == nil {
				s.voiceConn.Close(s.destroyCtx)
			}
		}

		select {
		case result := <-s.trackStreamSetupResultCh:
			if result.src != nil {
				result.src.Close()
			}
		default:
		}

		s.manager.detach(s.guildID)

		s.cancelDestroyCtx()
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
		case err := <-s.frameProvider.SourceDone():
			s.workerHandleTrackEnded(err)
		case task := <-s.trackEnqueueCh:
			s.workerHandleEnqueue(task)
		case task := <-s.trackSkipCh:
			s.workerHandleSkip(task)
		}
	}
}

func (s *Session) workerHandleDestroyRequest(reason DestroyReason) {
	s.destroyReason = reason
	s.cancelCtx()
}

func (s *Session) workerHandleInacitivityTimeout() {
	s.destroyReason = DestroyReasonInactivityTimeout
	s.cancelCtx()
}

func (s *Session) workerHandleConnOpenResult(err error) {
	if err != nil {
		s.cancelCtx()
		s.destroyReason = DestroyReasonOpenFailed
		s.logger.Error("Failed to open voice connection", slog.Any("err", err))
		return
	}
	s.voiceConn.SetOpusFrameProvider(s.frameProvider)

	s.voiceConnOpen = true
	s.logger.Debug("Opened voice connection")
	s.advanceQueue()
}

func (s *Session) workerHandleTrackStreamSetupResult(result trackStreamSetupResult) {
	s.trackStreamSetupCancelFn()
	s.trackStreamSetupCancelFn = nil

	if result.err != nil {
		go func(currentTrack media.Track) {
			if _, err := s.manager.client.Rest.CreateMessage(s.textChannelID, discord.MessageCreate{
				Content: fmt.Sprintf("%s **An unexpected error has occured when starting playback of `%s`**", messages.IconAppError, messages.EscapeMarkdown(currentTrack.Title())),
				Flags:   discord.MessageFlagSuppressNotifications,
			}, rest.WithCtx(s.ctx)); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("Failed to send playback setup error message", slog.Any("err", err))
			}
		}(s.currentTrack)
		s.logger.Error("Failed to set up track stream", slog.String("trackTitle", s.currentTrack.Title()), slog.Any("err", result.err))
		s.advanceQueue()
		return
	}

	s.frameProvider.SetSource(result.src)
	s.logger.Debug("Track stream setup done", slog.String("trackTitle", s.currentTrack.Title()))
}

func (s *Session) workerHandleTrackEnded(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		go func(currentTrack media.Track) {
			if _, err := s.manager.client.Rest.CreateMessage(s.textChannelID, discord.MessageCreate{
				Content: fmt.Sprintf("%s **An unexpected error has occured during playback of `%s`**", messages.IconAppError, messages.EscapeMarkdown(currentTrack.Title())),
				Flags:   discord.MessageFlagSuppressNotifications,
			}, rest.WithCtx(s.ctx)); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("Failed to send playback runtime error message", slog.Any("err", err))
			}
		}(s.currentTrack)
		s.logger.Error("Track ended with error", slog.String("trackTitle", s.currentTrack.Title()), slog.Any("err", err))
	} else {
		s.logger.Debug("Track ended", slog.String("trackTitle", s.currentTrack.Title()), slog.Any("err", err))
	}

	s.advanceQueue()
}

func (s *Session) workerHandleEnqueue(task trackEnqueueTask) {
	s.mu.Lock()
	s.queue = append(s.queue, task.tracks...)
	s.mu.Unlock()
	if s.currentTrack == nil {
		s.advanceQueue()
		task.doneCh <- true
	} else {
		task.doneCh <- false
	}
}

func (s *Session) workerHandleSkip(task trackSkipTask) {
	anythingToSkip := s.currentTrack != nil
	s.advanceQueue()
	task.doneCh <- anythingToSkip
}

func (s *Session) advanceQueue() {
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
			s.queue[0] = nil
			s.queue = s.queue[1:]
		} else {
			s.currentTrack = nil
		}
	}
	s.mu.Unlock()

	if s.currentTrack == nil {
		s.inactivityTimer.Reset(s.manager.inactivityTimeout)
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

	s.logger.Debug("Setting up track stream", slog.String("trackTitle", s.currentTrack.Title()))

	var trackStreamSetupCtx context.Context
	trackStreamSetupCtx, s.trackStreamSetupCancelFn = context.WithTimeout(s.ctx, 15*time.Second)

	go func() {
		s.trackStreamSetupResultCh <- func() trackStreamSetupResult {
			stream, err := s.currentTrack.Stream(trackStreamSetupCtx)
			if err != nil {
				return trackStreamSetupResult{err: err}
			}

			transcoder, err := transcode.NewTranscoder(stream, s.manager.transcoderOptions)
			if err != nil {
				stream.Close()
				return trackStreamSetupResult{err: err}
			}

			return trackStreamSetupResult{src: transcoder}
		}()
	}()
}
