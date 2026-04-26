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
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/musicbot/messages"
	"github.com/szczursonn/rythm5/internal/transcode"
)

type sessionPlayerEnqueueTask struct {
	tracks []media.Track
	doneCh chan EnqueueResult
}

type EnqueueResult struct {
	ImmediatePlayback bool
}

type sessionPlayerSkipTask struct {
	doneCh chan SkipResult
}

type SkipResult struct {
	AnythingToSkip bool
}

type sessionPlayerTranscoderSetupResult struct {
	transcoder *transcode.Transcoder
	err        error
}

type sessionPlayer struct {
	// set at creation
	s                       *Session
	enqueueTaskCh           chan sessionPlayerEnqueueTask
	skipTaskCh              chan sessionPlayerSkipTask
	inactivityTimer         *time.Timer
	transcoderSetupResultCh chan sessionPlayerTranscoderSetupResult
	transcoderDoneCh        chan error
	transcodersCloseWg      sync.WaitGroup

	// worker only
	transcoderSetupCancelFn context.CancelFunc

	// mutex-protected, but worker can read without lock
	mediaMu      sync.Mutex
	queue        []media.Track
	currentTrack media.Track
	looping      bool

	// full mutex-protected
	transcoderMu sync.Mutex
	transcoder   *transcode.Transcoder
	prevFrame    []byte
}

func newSessionPlayer(s *Session) *sessionPlayer {
	return &sessionPlayer{
		s:                       s,
		enqueueTaskCh:           make(chan sessionPlayerEnqueueTask),
		skipTaskCh:              make(chan sessionPlayerSkipTask),
		inactivityTimer:         time.NewTimer(s.manager.inactivityTimeout),
		transcoderSetupResultCh: make(chan sessionPlayerTranscoderSetupResult),
		transcoderDoneCh:        make(chan error, 1),
	}
}

func (sp *sessionPlayer) CurrentTrack() media.Track {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()
	return sp.currentTrack
}

func (sp *sessionPlayer) Queue() []media.Track {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()

	queue := make([]media.Track, len(sp.queue))
	copy(queue, sp.queue)
	return queue
}

func (sp *sessionPlayer) ClearQueue() {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()
	clear(sp.queue)
	sp.queue = sp.queue[:0]
}

func (sp *sessionPlayer) ShuffleQueue() {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()

	for i := len(sp.queue) - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		sp.queue[i], sp.queue[j] = sp.queue[j], sp.queue[i]
	}
}

func (sp *sessionPlayer) Looping() bool {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()
	return sp.looping
}

func (sp *sessionPlayer) SetLooping(on bool) {
	sp.mediaMu.Lock()
	defer sp.mediaMu.Unlock()
	sp.looping = on
}

func (sp *sessionPlayer) Enqueue(ctx context.Context, tracks []media.Track) (EnqueueResult, error) {
	doneCh := make(chan EnqueueResult, 1)

	select {
	case sp.enqueueTaskCh <- sessionPlayerEnqueueTask{
		tracks: tracks,
		doneCh: doneCh,
	}:
	case <-ctx.Done():
		return EnqueueResult{}, ctx.Err()
	case <-sp.s.ctx.Done():
		return EnqueueResult{}, ErrSessionDestroyed
	}

	select {
	case result := <-doneCh:
		return result, nil
	case <-ctx.Done():
		return EnqueueResult{}, ctx.Err()
	case <-sp.s.ctx.Done():
		return EnqueueResult{}, ErrSessionDestroyed
	}
}

func (sp *sessionPlayer) EnqueueTaskCh() chan sessionPlayerEnqueueTask {
	return sp.enqueueTaskCh
}

func (sp *sessionPlayer) WorkerHandleEnqueueTask(task sessionPlayerEnqueueTask) {
	sp.mediaMu.Lock()
	sp.queue = append(sp.queue, task.tracks...)
	sp.mediaMu.Unlock()
	if sp.currentTrack == nil {
		sp.advanceQueue()
		task.doneCh <- EnqueueResult{
			ImmediatePlayback: true,
		}
	} else {
		task.doneCh <- EnqueueResult{
			ImmediatePlayback: false,
		}
	}
}

func (sp *sessionPlayer) Skip(ctx context.Context) (SkipResult, error) {
	doneCh := make(chan SkipResult, 1)

	select {
	case sp.skipTaskCh <- sessionPlayerSkipTask{
		doneCh: doneCh,
	}:
	case <-ctx.Done():
		return SkipResult{}, ctx.Err()
	case <-sp.s.ctx.Done():
		return SkipResult{}, ErrSessionDestroyed
	}

	select {
	case result := <-doneCh:
		return result, nil
	case <-ctx.Done():
		return SkipResult{}, ctx.Err()
	case <-sp.s.ctx.Done():
		return SkipResult{}, ErrSessionDestroyed
	}
}

func (sp *sessionPlayer) SkipTaskCh() chan sessionPlayerSkipTask {
	return sp.skipTaskCh
}

func (sp *sessionPlayer) WorkerHandleSkipTask(task sessionPlayerSkipTask) {
	anythingToSkip := sp.currentTrack != nil
	sp.advanceQueue()
	task.doneCh <- SkipResult{
		AnythingToSkip: anythingToSkip,
	}
}

func (sp *sessionPlayer) InactivityTimerCh() <-chan time.Time {
	return sp.inactivityTimer.C
}

func (sp *sessionPlayer) WorkerHandleInactivityTimeout() {
	select {
	case sp.s.destroyReqCh <- DestroyReasonInactivityTimeout:
	default:
	}
	sp.s.cancelCtx()
}

func (sp *sessionPlayer) TranscoderSetupResultCh() chan sessionPlayerTranscoderSetupResult {
	return sp.transcoderSetupResultCh
}

func (sp *sessionPlayer) WorkerHandleTranscoderSetupResult(result sessionPlayerTranscoderSetupResult) {
	sp.transcoderSetupCancelFn()
	sp.transcoderSetupCancelFn = nil

	if result.err != nil {
		go func(currentTrack media.Track) {
			if _, err := sp.s.manager.client.Rest.CreateMessage(sp.s.notificationsTextChannelID, discord.MessageCreate{
				Content: fmt.Sprintf("%s **An error has occured when starting playback of %s**", messages.IconAppError, messages.MakeMarkdownLink(currentTrack.Title(), currentTrack.WebpageURL())),
				Flags:   discord.MessageFlagSuppressNotifications,
			}, rest.WithCtx(sp.s.ctx)); err != nil && !errors.Is(err, context.Canceled) {
				sp.s.logger.Error("Failed to send playback setup error message", slog.Any("err", err))
			}
		}(sp.currentTrack)
		sp.s.logger.Error("Failed to start session playback", slog.String("trackTitle", sp.currentTrack.Title()), slog.Any("err", result.err))
		sp.advanceQueue()
		return
	}

	sp.transcoderMu.Lock()
	sp.transcoder = result.transcoder
	sp.transcoderMu.Unlock()
	sp.s.logger.Debug("Session playback setup done", slog.String("trackTitle", sp.currentTrack.Title()))
}

func (sp *sessionPlayer) TranscoderDoneCh() chan error {
	return sp.transcoderDoneCh
}

func (sp *sessionPlayer) WorkerHandleTranscoderDone(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		go func(currentTrack media.Track) {
			if _, err := sp.s.manager.client.Rest.CreateMessage(sp.s.notificationsTextChannelID, discord.MessageCreate{
				Content: fmt.Sprintf("%s **An unexpected error has occured during playback of %s**", messages.IconAppError, messages.MakeMarkdownLink(currentTrack.Title(), currentTrack.WebpageURL())),
				Flags:   discord.MessageFlagSuppressNotifications,
			}, rest.WithCtx(sp.s.ctx)); err != nil && !errors.Is(err, context.Canceled) {
				sp.s.logger.Error("Failed to send playback runtime error message", slog.Any("err", err))
			}
		}(sp.currentTrack)
		sp.s.logger.Error("Session playback ended with error", slog.String("trackTitle", sp.currentTrack.Title()), slog.Any("err", err))
	} else {
		sp.s.logger.Debug("Session playback ended", slog.String("trackTitle", sp.currentTrack.Title()), slog.Any("err", err))
	}

	sp.advanceQueue()
}

func (sp *sessionPlayer) WorkerCleanup() {
	if sp.transcoderSetupCancelFn == nil {
		sp.transcoderMu.Lock()
		if sp.transcoder != nil {
			if sp.prevFrame != nil {
				sp.transcoder.ReleaseFrame(sp.prevFrame)
				sp.prevFrame = nil
			}
			sp.transcodersCloseWg.Go(sp.transcoder.Close)
		}
		sp.transcoderMu.Unlock()
	} else {
		sp.transcoderSetupCancelFn()

		if result := <-sp.transcoderSetupResultCh; result.err == nil {
			sp.transcodersCloseWg.Go(result.transcoder.Close)
		}
	}

	sp.transcodersCloseWg.Wait()
}

func (sp *sessionPlayer) advanceQueue() {
	if sp.transcoderSetupCancelFn != nil {
		sp.transcoderSetupCancelFn()
		// handler will call advanceQueue
		return
	}

	sp.transcoderMu.Lock()
	if sp.transcoder != nil {
		if sp.prevFrame != nil {
			sp.transcoder.ReleaseFrame(sp.prevFrame)
			sp.prevFrame = nil
		}
		sp.transcodersCloseWg.Go(sp.transcoder.Close)
		sp.transcoder = nil
	}
	sp.transcoderMu.Unlock()

	sp.mediaMu.Lock()
	if !sp.looping {
		if len(sp.queue) > 0 {
			sp.currentTrack = sp.queue[0]
			sp.queue = sp.queue[1:]
		} else {
			sp.currentTrack = nil
		}
	}
	sp.mediaMu.Unlock()

	if sp.currentTrack == nil {
		sp.resetInactivityTimeout()
		return
	}
	sp.stopInactivityTimeout()

	sp.s.logger.Debug("Setting up track stream", slog.String("trackTitle", sp.currentTrack.Title()))

	ctx, cancelCtx := context.WithTimeout(sp.s.ctx, sp.s.manager.trackSetupTimeout)
	sp.transcoderSetupCancelFn = cancelCtx

	go func() {
		sp.transcoderSetupResultCh <- func() sessionPlayerTranscoderSetupResult {
			defer cancelCtx()

			stream, err := sp.currentTrack.Stream(ctx)
			if err != nil {
				return sessionPlayerTranscoderSetupResult{err: err}
			}

			transcoder, err := transcode.NewTranscoder(stream, sp.s.manager.transcoderOptions)
			if err != nil {
				stream.Close()
				return sessionPlayerTranscoderSetupResult{err: err}
			}

			return sessionPlayerTranscoderSetupResult{transcoder: transcoder}
		}()
	}()
}

func (sp *sessionPlayer) resetInactivityTimeout() {
	sp.stopInactivityTimeout()
	sp.inactivityTimer.Reset(sp.s.manager.inactivityTimeout)
	sp.s.logger.Debug("Session inactivity timer reset")
}

func (sp *sessionPlayer) stopInactivityTimeout() {
	if !sp.inactivityTimer.Stop() {
		select {
		case <-sp.inactivityTimer.C:
		default:
		}
		sp.s.logger.Debug("Session inactivity timer stopped")
	}
}

// impl: voice.OpusFrameProvider

func (sp *sessionPlayer) ProvideOpusFrame() ([]byte, error) {
	sp.transcoderMu.Lock()
	defer sp.transcoderMu.Unlock()

	if sp.transcoder == nil {
		return nil, nil
	}

	if sp.prevFrame != nil {
		sp.transcoder.ReleaseFrame(sp.prevFrame)
		sp.prevFrame = nil
	}

	select {
	case frame, ok := <-sp.transcoder.Frames():
		if !ok {
			sp.transcoderDoneCh <- sp.transcoder.Err()
			sp.transcodersCloseWg.Go(sp.transcoder.Close)
			sp.transcoder = nil
			return nil, nil
		}

		sp.prevFrame = frame
		return frame, nil
	default:
		return nil, nil
	}
}

func (sp *sessionPlayer) Close() {
	// called when voice connection closes
	// ignore - player is independent from voice connection
}
