package audiotranscoderbuf

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/audio/audiotranscoder"
	"github.com/szczursonn/rythm5/internal/bufferpool"
)

const errPrefix = "audio/transcoderbuf: "

const DefaultBufferDuration = 15 * time.Second

var ErrFrameNotReady = fmt.Errorf("")

type BufferedTranscoderOptions struct {
	audiotranscoder.TranscoderOptions
	BufferDuration time.Duration
}

type BufferedTranscoder struct {
	transcoder       *audiotranscoder.Transcoder
	frames           chan []byte
	err              error
	shutdownWorkerCh chan struct{}
	closeOnce        sync.Once
}

var bufPool = bufferpool.New(512)

func NewBufferedTranscoder(src io.ReadCloser, opts BufferedTranscoderOptions) (*BufferedTranscoder, error) {
	if opts.BufferDuration <= 0 {
		opts.BufferDuration = DefaultBufferDuration
	}

	transcoder, err := audiotranscoder.NewTranscoder(src, opts.TranscoderOptions)
	if err != nil {
		return nil, err
	}

	bt := &BufferedTranscoder{
		transcoder:       transcoder,
		frames:           make(chan []byte, max(int(opts.BufferDuration/audiotranscoder.OpusFrameDuration), 1)),
		shutdownWorkerCh: make(chan struct{}),
	}
	go bt.worker()
	return bt, nil
}

func (bt *BufferedTranscoder) ReadAvailableOpusFrame() ([]byte, error) {
	select {
	case frame, ok := <-bt.frames:
		if ok {
			return frame, nil
		}
		return nil, bt.err
	default:
		return nil, ErrFrameNotReady
	}
}

func (bt *BufferedTranscoder) ReadOpusFrame() ([]byte, error) {
	frame, ok := <-bt.frames
	if !ok {
		return nil, bt.err
	}
	return frame, nil
}

func (_ *BufferedTranscoder) ReleaseOpusFrame(frame []byte) {
	bufPool.Put(frame)
}

func (bt *BufferedTranscoder) Close() {
	bt.closeOnce.Do(func() {
		close(bt.shutdownWorkerCh)
		for frame := range bt.frames {
			bufPool.Put(frame)
		}
		bt.transcoder.Close()
	})
}

func (bt *BufferedTranscoder) worker() {
	defer close(bt.frames)
	for {
		buf := bufPool.Get()
		packet, err := bt.transcoder.ReadOpusFrame(buf)
		if err != nil {
			bufPool.Put(buf)
			bt.err = err
			return
		}

		select {
		case bt.frames <- packet:
		case <-bt.shutdownWorkerCh:
			bt.err = fmt.Errorf(errPrefix + "closed")
			bufPool.Put(packet)
			return
		}
	}
}
