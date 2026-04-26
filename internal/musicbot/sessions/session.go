package sessions

import (
	"context"
	"errors"
	"log/slog"

	"github.com/disgoorg/snowflake/v2"
	"github.com/szczursonn/rythm5/internal/media"
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
	manager                    *Manager
	logger                     *slog.Logger
	svcm                       *sessionVoiceConnManager
	sp                         *sessionPlayer
	guildID                    snowflake.ID
	notificationsTextChannelID snowflake.ID
	ctx                        context.Context
	cancelCtx                  context.CancelFunc
	destroyReqCh               chan DestroyReason
	destroyDoneCh              chan struct{}

	// worker-only
	destroyReason DestroyReason
}

func newSession(m *Manager, guildID snowflake.ID, notificationsTextChannelID snowflake.ID, voiceChannelID snowflake.ID) *Session {
	s := &Session{
		manager:                    m,
		logger:                     m.logger.With(slog.String("guildID", guildID.String())),
		guildID:                    guildID,
		notificationsTextChannelID: notificationsTextChannelID,
		destroyReqCh:               make(chan DestroyReason, 1),
		destroyDoneCh:              make(chan struct{}),
	}
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	s.svcm = newSessionVoiceConnManager(s, voiceChannelID)
	s.sp = newSessionPlayer(s)
	s.logger.Info("Session created")
	go s.worker()
	return s
}

func (s *Session) CurrentTrack() media.Track {
	return s.sp.CurrentTrack()
}

func (s *Session) Queue() []media.Track {
	return s.sp.Queue()
}

func (s *Session) ClearQueue() {
	s.sp.ClearQueue()
}

func (s *Session) ShuffleQueue() {
	s.sp.ShuffleQueue()
}

func (s *Session) Looping() bool {
	return s.sp.Looping()
}

func (s *Session) SetLooping(on bool) {
	s.sp.SetLooping(on)
}

func (s *Session) Enqueue(ctx context.Context, tracks []media.Track) (EnqueueResult, error) {
	return s.sp.Enqueue(ctx, tracks)
}

func (s *Session) Skip(ctx context.Context) (SkipResult, error) {
	return s.sp.Skip(ctx)
}

func (s *Session) Destroy() {
	select {
	case s.destroyReqCh <- DestroyReasonRequested:
	default:
	}

	<-s.destroyDoneCh
}

func (s *Session) handleVoiceStateUpdate(channelID *snowflake.ID) {
	s.svcm.HandleVoiceStateUpdate(channelID)
}

func (s *Session) worker() {
	defer func() {
		s.cancelCtx()
		s.svcm.WorkerCleanup()
		s.sp.WorkerCleanup()

		s.manager.detach(s.guildID)

		s.logger.Info("Session destroyed", slog.String("reason", string(s.destroyReason)))
		close(s.destroyDoneCh)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case reason := <-s.destroyReqCh:
			s.workerHandleDestroyRequest(reason)
		case err := <-s.svcm.WorkerOpenResultCh():
			s.svcm.WorkerHandleOpenResult(err)
		case task := <-s.sp.EnqueueTaskCh():
			s.sp.WorkerHandleEnqueueTask(task)
		case task := <-s.sp.SkipTaskCh():
			s.sp.WorkerHandleSkipTask(task)
		case <-s.sp.InactivityTimerCh():
			s.sp.WorkerHandleInactivityTimeout()
		case result := <-s.sp.TranscoderSetupResultCh():
			s.sp.WorkerHandleTranscoderSetupResult(result)
		case err := <-s.sp.TranscoderDoneCh():
			s.sp.WorkerHandleTranscoderDone(err)
		}
	}
}

func (s *Session) workerHandleDestroyRequest(reason DestroyReason) {
	s.destroyReason = reason
	s.cancelCtx()
}
