package logfile

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"
)

const errPrefix = "logging/logfile: "

type bufferedLogFileWriter struct {
	ctx          context.Context
	cancelCtx    context.CancelFunc
	workerDoneCh chan struct{}

	file *os.File
	buff *bufio.Writer
}

type Options struct {
	Path          string
	BufferSize    int
	FlushInterval time.Duration
}

func NewBufferedLogFile(opts Options) (io.WriteCloser, error) {
	if opts.Path == "" {
		panic(errPrefix + "path is required")
	}

	if opts.FlushInterval <= 0 {
		opts.FlushInterval = time.Second
	}

	if opts.BufferSize <= 0 {
		opts.BufferSize = 65536
	}

	file, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	blfw := &bufferedLogFileWriter{
		workerDoneCh: make(chan struct{}),
		file:         file,
		buff:         bufio.NewWriterSize(file, opts.BufferSize),
	}
	blfw.ctx, blfw.cancelCtx = context.WithCancel(context.Background())

	go blfw.flushWorker(opts.FlushInterval)

	return blfw, nil
}

func (blfw *bufferedLogFileWriter) Write(p []byte) (n int, err error) {
	return blfw.buff.Write(p)
}

func (blfw *bufferedLogFileWriter) Close() error {
	blfw.cancelCtx()
	<-blfw.workerDoneCh

	return blfw.file.Close()
}

func (blfw *bufferedLogFileWriter) flushWorker(flushInterval time.Duration) {
	defer close(blfw.workerDoneCh)
	defer blfw.buff.Flush()

	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-blfw.ctx.Done():
			return
		case <-flushTicker.C:
		}

		blfw.buff.Flush()
	}
}
