package transcode

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/bufferpool"
	"github.com/szczursonn/rythm5/internal/proclimit"
	"github.com/szczursonn/rythm5/internal/transcode/oggdemux"
)

const errPrefix = "transcode: "

const opusFrameDuration = 20 * time.Millisecond
const ffmpegMaxShutdownDuration = 5 * time.Second

const (
	defaultFfmpegPath     = "ffmpeg"
	defaultBitrate        = 96
	defaultBufferDuration = 15 * time.Second
)

var bufPool = bufferpool.New(512)

var ErrClosed = errors.New(errPrefix + "closed")

type Options struct {
	FfmpegPath        string
	Bitrate           int
	BufferDuration    time.Duration
	CPUPriority       proclimit.CPUPriority
	OOMKillerPriority proclimit.OOMKillerPriority
}

func (opts *Options) applyDefaults() {
	if opts.FfmpegPath == "" {
		opts.FfmpegPath = defaultFfmpegPath
	}
	if opts.Bitrate <= 0 {
		opts.Bitrate = defaultBitrate
	}
	if opts.BufferDuration <= 0 {
		opts.BufferDuration = defaultBufferDuration
	}
}

type Transcoder struct {
	cmd    *exec.Cmd
	src    io.ReadCloser
	frames chan []byte

	doneCh    chan struct{}
	closeOnce sync.Once

	errMu sync.Mutex
	err   error
}

func NewTranscoder(src io.ReadCloser, opts Options) (*Transcoder, error) {
	opts.applyDefaults()

	cmd := exec.Command(opts.FfmpegPath,
		"-i", "pipe:0",
		"-vn",
		"-c:a", "libopus",
		"-b:a", fmt.Sprintf("%dk", opts.Bitrate),
		"-application", "audio",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", fmt.Sprint(opusFrameDuration.Milliseconds()),
		"-f", "ogg",
		"-loglevel", "error",
		"pipe:1",
	)
	cmd.WaitDelay = ffmpegMaxShutdownDuration

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"creating ffmpeg stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf(errPrefix+"creating ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdoutPipe.Close()
		stdinPipe.Close()
		return nil, fmt.Errorf(errPrefix+"starting ffmpeg process: %w", err)
	}

	if opts.OOMKillerPriority != proclimit.OOMKillerPriorityUnset {
		proclimit.ApplyOOMKillerPriority(cmd.Process.Pid, opts.OOMKillerPriority)
	}
	if opts.CPUPriority != proclimit.CPUPriorityUnset {
		proclimit.ApplyCPUPriority(cmd.Process.Pid, opts.CPUPriority)
	}

	t := &Transcoder{
		cmd:    cmd,
		src:    src,
		frames: make(chan []byte, max(int(opts.BufferDuration/opusFrameDuration), 1)),
		doneCh: make(chan struct{}),
	}

	go t.pumpSource(stdinPipe)
	go t.pumpFrames(oggdemux.NewOggOpusPacketReader(stdoutPipe))

	return t, nil
}

func (t *Transcoder) Frames() <-chan []byte {
	return t.frames
}

func (t *Transcoder) Err() error {
	t.errMu.Lock()
	defer t.errMu.Unlock()
	return t.err
}

func (*Transcoder) ReleaseFrame(frame []byte) {
	bufPool.Put(frame)
}

func (t *Transcoder) Close() {
	t.closeOnce.Do(func() {
		close(t.doneCh)
		t.cmd.Process.Kill()
		t.src.Close()
		for frame := range t.frames {
			bufPool.Put(frame)
		}
		t.cmd.Wait()
	})
}

func (t *Transcoder) pumpSource(stdinPipe io.WriteCloser) {
	defer stdinPipe.Close()
	if _, err := io.Copy(stdinPipe, t.src); err != nil {
		t.errMu.Lock()
		defer t.errMu.Unlock()
		if t.err == nil {
			t.err = fmt.Errorf(errPrefix+"writing to ffmpeg stdin: %w", err)
		}
	}
}

func (t *Transcoder) pumpFrames(demuxer *oggdemux.OggOpusDemuxer) {
	defer close(t.frames)

	for {
		buf := bufPool.Get()
		packet, err := demuxer.ReadAudioPacket(buf)
		if err != nil {
			bufPool.Put(buf)

			t.errMu.Lock()
			defer t.errMu.Unlock()

			select {
			case <-t.doneCh:
				t.err = ErrClosed
			default:
				if t.err == nil || errors.Is(t.err, io.EOF) {
					t.err = err
				}
			}

			return
		}

		select {
		case t.frames <- packet:
		case <-t.doneCh:
			bufPool.Put(packet)
			t.errMu.Lock()
			defer t.errMu.Unlock()
			t.err = ErrClosed
			return
		}
	}
}
