package httpaudio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
)

type rangeReader struct {
	client        *Client
	url           string
	httpHeaders   map[string]string
	contentLength int

	ctx       context.Context
	cancelCtx context.CancelFunc

	nextFetchOffset int
	buf             []byte
	bufStartPos     int
	bufEndPos       int
}

var _ io.ReadCloser = (*rangeReader)(nil)

func newRangeReader(client *Client, headResp *http.Response, url string, headers map[string]string) (*rangeReader, error) {
	if !slices.Contains(headResp.Header.Values("Accept-Ranges"), "bytes") {
		return nil, fmt.Errorf(errPrefix + "server does not accept \"bytes\" range requests")
	}

	contentLengthStr := headResp.Header.Get("Content-Length")
	contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil || contentLength <= 0 {
		return nil, fmt.Errorf(errPrefix+"server did not provide valid content length: %s", contentLengthStr)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	return &rangeReader{
		client:        client,
		url:           url,
		httpHeaders:   headers,
		contentLength: int(contentLength),
		ctx:           ctx,
		cancelCtx:     cancelCtx,
		buf:           make([]byte, client.chunkSize),
	}, nil
}

func (rr *rangeReader) Read(p []byte) (int, error) {
	if rr.buf == nil {
		return 0, fmt.Errorf(errPrefix + "rangereader is closed")
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

	ctx, cancelCtx := context.WithTimeout(rr.ctx, rr.client.chunkFetchTimeout)
	defer cancelCtx()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rr.url, nil)
	if err != nil {
		return fmt.Errorf(errPrefix+"rangereader GET request create: %w", err)
	}
	for k, v := range rr.httpHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startOffset, endOffset))

	resp, err := rr.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(errPrefix+"rangereader GET request failed for range %d-%d: %w", startOffset, endOffset, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf(errPrefix+"rangereader GET returned unexpected status %d for range %d-%d", resp.StatusCode, startOffset, endOffset)
	}

	rr.bufStartPos = 0
	n, err := io.ReadFull(resp.Body, rr.buf)
	rr.nextFetchOffset += n
	rr.bufEndPos = n
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf(errPrefix+"rangereader GET reading body for range %d-%d: %w", startOffset, endOffset, err)
	}

	return nil
}
