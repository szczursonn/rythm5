package logfile

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"
	"time"
)

type bufferedLogFileWriter struct {
	ctx       context.Context
	cancelCtx context.CancelFunc
	wg        sync.WaitGroup

	file *os.File
	buff *bufio.Writer
}

func NewBufferedLogFile(logFilePath string, bufferSize int, flushInterval time.Duration) (io.WriteCloser, error) {
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	ctx, cancelCtx := context.WithCancel(context.Background())

	writer := &bufferedLogFileWriter{
		ctx:       ctx,
		cancelCtx: cancelCtx,
		file:      file,
		buff:      bufio.NewWriterSize(file, bufferSize),
	}

	writer.wg.Add(1)
	go writer.flushWorker(flushInterval)

	return writer, nil
}

func (blfw *bufferedLogFileWriter) Write(p []byte) (n int, err error) {
	return blfw.buff.Write(p)
}

func (blfw *bufferedLogFileWriter) Close() error {
	blfw.cancelCtx()
	blfw.wg.Wait()

	return blfw.file.Close()
}

func (blfw *bufferedLogFileWriter) flushWorker(flushInterval time.Duration) {
	defer blfw.wg.Done()
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	running := true
	for running {
		select {
		case <-blfw.ctx.Done():
			running = false
		case <-flushTicker.C:
		}

		blfw.buff.Flush()
	}
}
