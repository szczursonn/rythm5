package disgoframe

import (
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/szczursonn/rythm5/internal/transcode"
)

type FrameProvider struct {
	mu           sync.Mutex
	transcoder   *transcode.Transcoder
	sourceDoneCh chan error
	prevFrame    []byte
}

var _ voice.OpusFrameProvider = (*FrameProvider)(nil)

func NewFrameProvider() *FrameProvider {
	return &FrameProvider{
		sourceDoneCh: make(chan error, 1),
	}
}

func (fp *FrameProvider) ProvideOpusFrame() ([]byte, error) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.transcoder == nil {
		return nil, nil
	}

	if fp.prevFrame != nil {
		fp.transcoder.ReleaseFrame(fp.prevFrame)
		fp.prevFrame = nil
	}

	select {
	case frame, ok := <-fp.transcoder.Frames():
		if !ok {
			fp.sourceDoneCh <- fp.transcoder.Err()
			fp.transcoder.Close()
			fp.transcoder = nil
			return nil, nil
		}

		fp.prevFrame = frame
		return frame, nil
	default:
		return nil, nil
	}
}

func (fp *FrameProvider) SetSource(newTranscoder *transcode.Transcoder) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if fp.transcoder != nil {
		if fp.prevFrame != nil {
			fp.transcoder.ReleaseFrame(fp.prevFrame)
			fp.prevFrame = nil
		}
		go fp.transcoder.Close()
	}

	fp.transcoder = newTranscoder
}

func (fp *FrameProvider) SourceDone() <-chan error {
	return fp.sourceDoneCh
}

func (fp *FrameProvider) Close() {
	fp.SetSource(nil)
}
