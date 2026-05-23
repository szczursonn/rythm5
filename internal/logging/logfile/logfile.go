package logfile

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

const errPrefix = "logging/logfile: "

type bufferedLogFile struct {
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
		return nil, fmt.Errorf(errPrefix + "path is required")
	}

	if opts.FlushInterval <= 0 {
		opts.FlushInterval = time.Second
	}

	if opts.BufferSize <= 0 {
		opts.BufferSize = 65536
	}

	file, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"opening file: %w", err)
	}

	blf := &bufferedLogFile{
		workerDoneCh: make(chan struct{}),
		file:         file,
		buff:         bufio.NewWriterSize(file, opts.BufferSize),
	}
	blf.ctx, blf.cancelCtx = context.WithCancel(context.Background())

	go blf.flushWorker(opts.FlushInterval)

	return blf, nil
}

func (blf *bufferedLogFile) Write(p []byte) (n int, err error) {
	return blf.buff.Write(p)
}

func (blf *bufferedLogFile) Close() error {
	blf.cancelCtx()
	<-blf.workerDoneCh

	return blf.file.Close()
}

func (blf *bufferedLogFile) flushWorker(flushInterval time.Duration) {
	defer close(blf.workerDoneCh)
	defer blf.buff.Flush()

	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-blf.ctx.Done():
			return
		case <-flushTicker.C:
		}

		blf.buff.Flush()
	}
}
