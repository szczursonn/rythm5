package audiotranscoder

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/szczursonn/rythm5/internal/audio/audiooggdemux"
	"github.com/szczursonn/rythm5/internal/proclimit"
)

const errPrefix = "audio/transcoder: "

const OpusFrameDuration = 20 * time.Millisecond
const DefaultBitrate = 96

type TranscoderOptions struct {
	FfmpegPath string
	Bitrate    int
}

type Transcoder struct {
	demuxer   *audiooggdemux.OggOpusDemuxer
	cmd       *exec.Cmd
	src       io.ReadCloser
	closeOnce sync.Once

	srcErrMu sync.Mutex
	srcErr   error
}

func NewTranscoder(src io.ReadCloser, opts TranscoderOptions) (*Transcoder, error) {
	if opts.FfmpegPath == "" {
		opts.FfmpegPath = "ffmpeg"
	}
	if opts.Bitrate <= 0 {
		opts.Bitrate = DefaultBitrate
	}

	cmd := exec.Command(opts.FfmpegPath,
		"-i", "pipe:0",
		"-vn",
		"-c:a", "libopus",
		"-b:a", fmt.Sprintf("%dk", opts.Bitrate),
		"-application", "audio",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", fmt.Sprint(OpusFrameDuration.Milliseconds()),
		"-f", "ogg",
		"-loglevel", "error",
		"pipe:1",
	)
	cmd.WaitDelay = 5 * time.Second

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

	proclimit.ApplyOOMKillerPriority(cmd.Process.Pid, proclimit.OOMKillerPriorityAboveNormal)
	proclimit.ApplyCPUPriority(cmd.Process.Pid, proclimit.CPUPriorityLow)

	t := &Transcoder{
		demuxer: audiooggdemux.NewOggOpusPacketReader(stdoutPipe),
		cmd:     cmd,
		src:     src,
	}

	go func() {
		defer stdinPipe.Close()
		_, err := io.Copy(stdinPipe, src)
		if err != nil {
			t.srcErrMu.Lock()
			defer t.srcErrMu.Unlock()
			t.srcErr = fmt.Errorf(errPrefix+"writing to ffmpeg stdin: %w", err)
		}
	}()

	return t, nil
}

func (t *Transcoder) ReadOpusFrame(dst []byte) (buf []byte, err error) {
	buf, err = t.demuxer.ReadAudioPacket(dst)
	if err != nil && !errors.Is(err, io.EOF) {
		t.srcErrMu.Lock()
		defer t.srcErrMu.Unlock()
		if t.srcErr != nil && !errors.Is(t.srcErr, io.EOF) {
			err = t.srcErr
		}
	}

	return
}

func (t *Transcoder) Close() {
	t.closeOnce.Do(func() {
		t.cmd.Process.Kill()
		t.src.Close()
		t.cmd.Wait()
	})
}
