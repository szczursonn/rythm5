package rangeread

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"
)

const errPrefix = "rangeread: "

const DefaultChunkSize = 1 << 20
const DefaultChunkFetchTimeout = 10 * time.Second

type rangeReader struct {
	url               string
	client            *http.Client
	httpHeaders       map[string]string
	chunkFetchTimeout time.Duration
	contentLength     int

	ctx       context.Context
	cancelCtx context.CancelFunc

	nextFetchOffset int
	buf             []byte
	bufStartPos     int
	bufEndPos       int
}

type RangeReaderOptions struct {
	HTTPClient        *http.Client
	URL               string
	HTTPHeaders       map[string]string
	ChunkSize         int
	ChunkFetchTimeout time.Duration
}

func NewRangeReader(ctx context.Context, options RangeReaderOptions) (io.ReadCloser, error) {
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	if options.HTTPHeaders == nil {
		options.HTTPHeaders = map[string]string{}
	}
	if options.ChunkSize <= 0 {
		options.ChunkSize = DefaultChunkSize
	}
	if options.ChunkFetchTimeout <= 0 {
		options.ChunkFetchTimeout = DefaultChunkFetchTimeout
	}

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, options.URL, nil)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"HEAD request creation failed: %w", err)
	}

	for k, v := range options.HTTPHeaders {
		headReq.Header.Set(k, v)
	}

	resp, err := options.HTTPClient.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"HEAD request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errPrefix+"HEAD returned unexpected status %d", resp.StatusCode)
	}

	if !slices.Contains(resp.Header.Values("Accept-Ranges"), "bytes") {
		return nil, fmt.Errorf(errPrefix + "server does not support bytes range requests")
	}

	contentLengthStr := resp.Header.Get("Content-Length")
	contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil || contentLength <= 0 {
		return nil, fmt.Errorf(errPrefix+"server did not provide valid content length: %s", contentLengthStr)
	}

	readerCtx, readerCtxCancel := context.WithCancel(context.Background())

	r := &rangeReader{
		url:               options.URL,
		client:            options.HTTPClient,
		httpHeaders:       options.HTTPHeaders,
		chunkFetchTimeout: options.ChunkFetchTimeout,
		contentLength:     int(contentLength),
		ctx:               readerCtx,
		cancelCtx:         readerCtxCancel,
		buf:               make([]byte, options.ChunkSize),
	}

	return r, nil
}

func (rr *rangeReader) Read(p []byte) (int, error) {
	if rr.buf == nil {
		return 0, fmt.Errorf(errPrefix + "reader is closed")
	}

	if rr.nextFetchOffset == 0 || rr.bufStartPos == rr.bufEndPos {
		if err := rr.fetchNextChunk(); err != nil {
			return 0, err
		}
	}

	n := copy(p, rr.buf[rr.bufStartPos:rr.bufEndPos])
	rr.bufStartPos += n
	return n, nil
}

func (rr *rangeReader) Close() error {
	rr.cancelCtx()
	rr.buf = nil
	return nil
}

func (rr *rangeReader) fetchNextChunk() error {
	if rr.nextFetchOffset == rr.contentLength {
		return io.EOF
	}

	startOffset := rr.nextFetchOffset
	endOffset := min(startOffset+len(rr.buf), rr.contentLength) - 1

	ctx, cancelCtx := context.WithTimeout(rr.ctx, rr.chunkFetchTimeout)
	defer cancelCtx()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rr.url, nil)
	if err != nil {
		return fmt.Errorf(errPrefix+"chunk request create: %w", err)
	}
	for k, v := range rr.httpHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startOffset, endOffset))

	resp, err := rr.client.Do(req)
	if err != nil {
		return fmt.Errorf(errPrefix+"chunk request failed for range %d-%d: %w", startOffset, endOffset, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf(errPrefix+"chunk response unexpected status %d for range %d-%d", resp.StatusCode, startOffset, endOffset)
	}

	rr.bufStartPos = 0
	n, err := io.ReadFull(resp.Body, rr.buf)
	rr.nextFetchOffset += n
	rr.bufEndPos = n
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf(errPrefix+"chunk response reading body for range %d-%d: %w", startOffset, endOffset, err)
	}

	if n == 0 {
		return io.EOF
	}

	return nil
}

var _ io.ReadCloser = (*rangeReader)(nil)
