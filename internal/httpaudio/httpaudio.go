package httpaudio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
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
		return nil, fmt.Errorf(errPrefix+"HEAD \"%s\" request creation failed: %w", url, err)
	}
	for k, v := range headers {
		headReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"HEAD \"%s\" with headers %s request failed: %w", url, headersErrorString(headReq.Header), err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errPrefix+"HEAD \"%s\" with headers %s returned unexpected status %d", url, headersErrorString(headReq.Header), resp.StatusCode)
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

func headersErrorString(headers http.Header) string {
	var sb strings.Builder

	first := true
	for k := range headers {
		if first {
			first = false
		} else {
			sb.WriteString(",")
		}

		sb.WriteString("\"")
		sb.WriteString(k)
		sb.WriteString("\"=\"")
		sb.WriteString(headers.Get(k))
		sb.WriteString("\"")
	}

	return sb.String()
}
