package sessions

import (
	"context"
	"errors"
	"log/slog"

	"github.com/disgoorg/snowflake/v2"
)

type sessionVoiceConnManager struct {
	s *Session

	// worker-only
	openResultCh chan error
}

func newSessionVoiceConnManager(s *Session, initialVoiceChannelID snowflake.ID) *sessionVoiceConnManager {
	svcm := &sessionVoiceConnManager{
		s:            s,
		openResultCh: make(chan error, 1),
	}

	go func() {
		svcm.s.logger.Debug("Opening voice connection...")
		// context.Background(), because cancelling the context just makes Open return immediately and voice connection is simply left there in an unknown state for some fucking reason
		svcm.openResultCh <- svcm.s.manager.client.VoiceManager.CreateConn(svcm.s.guildID).Open(context.Background(), initialVoiceChannelID, false, true)
	}()

	return svcm
}

func (svcm *sessionVoiceConnManager) WorkerOpenResultCh() chan error {
	return svcm.openResultCh
}

func (svcm *sessionVoiceConnManager) WorkerHandleOpenResult(err error) {
	svcm.openResultCh = nil

	voiceConn := svcm.s.manager.client.VoiceManager.GetConn(svcm.s.guildID)
	if voiceConn == nil && err == nil {
		err = errors.New(errPrefix + "voice conn closed before open result was handled")
	}

	if err != nil {
		svcm.s.logger.Error("Failed to open voice connection", slog.Any("err", err))
		select {
		case svcm.s.destroyReqCh <- DestroyReasonOpenFailed:
		default:
		}
		return
	}

	voiceConn.SetOpusFrameProvider(svcm.s.sp)
	svcm.s.logger.Debug("Opened voice connection")
}

func (svcm *sessionVoiceConnManager) HandleVoiceStateUpdate(channelID *snowflake.ID) {
	if channelID == nil {
		select {
		case svcm.s.destroyReqCh <- DestroyReasonVoiceDisconnected:
		default:
		}
	}
}

func (svcm *sessionVoiceConnManager) WorkerCleanup() {
	if svcm.openResultCh != nil {
		<-svcm.openResultCh
	}

	voiceConn := svcm.s.manager.client.VoiceManager.GetConn(svcm.s.guildID)
	if voiceConn != nil {
		svcm.s.logger.Debug("Closing voice conn in cleanup")
		voiceConn.Close(context.Background())
		svcm.s.logger.Debug("Closed voice conn in cleanup")
	}
}
