package httpaudio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"
)

const errPrefix = "httpaudio: "

var ErrM3U8NotSupported = errors.New(errPrefix + "m3u8 streams are not yet supported")

type Client struct {
	httpClient        *http.Client
	chunkSize         int
	chunkFetchTimeout time.Duration
}

type ClientOptions struct {
	HTTPClient        *http.Client
	ChunkSize         int
	ChunkFetchTimeout time.Duration
}

func NewClient(opts ClientOptions) *Client {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1 << 20
	}
	if opts.ChunkFetchTimeout <= 0 {
		opts.ChunkFetchTimeout = 10 * time.Second
	}
	return &Client{
		httpClient:        opts.HTTPClient,
		chunkSize:         opts.ChunkSize,
		chunkFetchTimeout: opts.ChunkFetchTimeout,
	}
}

func (c *Client) Open(ctx context.Context, url string, headers map[string]string) (io.ReadCloser, error) {
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"HEAD request creation failed: %w", err)
	}
	for k, v := range headers {
		headReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"HEAD request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errPrefix+"HEAD returned unexpected status %d", resp.StatusCode)
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch mediaType {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl":
		return nil, ErrM3U8NotSupported
	}

	rr, err := newRangeReader(c, resp, url, headers)
	if err != nil {
		return nil, err
	}

	return rr, nil
}
