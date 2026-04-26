package safeudpconn

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave"
)

func NewUDPConn(daveSession godave.Session, ssrcLookup voice.SsrcLookupFunc, opts ...voice.UDPConnConfigOpt) voice.UDPConn {
	return &safeUDPConn{
		UDPConn: voice.NewUDPConn(daveSession, ssrcLookup, opts...),
	}
}

// fix for a bug in disgo
// voice.UDPConn.Close is called as part of voice.Conn.Close, first indirectly via voice.Conn.HandleVoiceStateUpdate callback and then as part of defer
// voice.UDPConn.Close does not check if voice.UDPConn.conn is nil, causing panic
type safeUDPConn struct {
	voice.UDPConn

	mu        sync.Mutex
	closeable bool
}

func (s *safeUDPConn) Open(ctx context.Context, ip string, port int, ssrc uint32) (string, int, error) {
	addr, p, err := s.UDPConn.Open(ctx, ip, port, ssrc)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil || !isDialError(err) {
		s.closeable = true
	}
	return addr, p, err
}

func (s *safeUDPConn) Close() error {
	s.mu.Lock()
	closeable := s.closeable
	s.mu.Unlock()
	if !closeable {
		return nil
	}
	return s.UDPConn.Close()
}

func isDialError(err error) bool {
	if opErr, ok := errors.AsType[*net.OpError](err); ok && opErr.Op == "dial" {
		return true
	}
	return false
}
