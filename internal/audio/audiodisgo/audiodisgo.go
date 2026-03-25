package audiodisgo

import (
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/szczursonn/rythm5/internal/audio/audiotranscoderbuf"
)

type FrameProvider struct {
	mu          sync.Mutex
	transcoder  *audiotranscoderbuf.BufferedTranscoder
	sourceErrCh chan error
	prevFrame   []byte
}

var _ voice.OpusFrameProvider = (*FrameProvider)(nil)

func NewFrameProvider(sourceErrCh chan error) *FrameProvider {
	return &FrameProvider{
		sourceErrCh: sourceErrCh,
	}
}

func (fp *FrameProvider) ProvideOpusFrame() ([]byte, error) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.transcoder == nil {
		return nil, nil
	}

	if fp.prevFrame != nil {
		fp.transcoder.ReleaseOpusFrame(fp.prevFrame)
		fp.prevFrame = nil
	}

	frame, err := fp.transcoder.ReadAvailableOpusFrame()
	if err != nil {
		if err != audiotranscoderbuf.ErrFrameNotReady {
			fp.transcoder.Close()
			fp.transcoder = nil
			fp.sourceErrCh <- err
		}

		return nil, nil
	}

	fp.prevFrame = frame
	return frame, nil
}

func (fp *FrameProvider) SetSource(newTranscoder *audiotranscoderbuf.BufferedTranscoder) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.transcoder != nil {
		if fp.prevFrame != nil {
			fp.transcoder.ReleaseOpusFrame(fp.prevFrame)
		}
		go fp.transcoder.Close()
	}
	fp.prevFrame = nil
	fp.transcoder = newTranscoder
}

func (fp *FrameProvider) Close() {
	fp.SetSource(nil)
}
