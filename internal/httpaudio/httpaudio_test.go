package httpaudio_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/szczursonn/rythm5/internal/httpaudio"
)

// ⚠️⚠️⚠️ AI-generated ⚠️⚠️⚠️

func newDirectAudioServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			if rangeHeader == "" {
				t.Error("GET request missing Range header")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var start, end int
			_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if err != nil {
				t.Errorf("invalid Range header: %s", rangeHeader)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if start < 0 || end >= len(content) || start > end {
				t.Errorf("Range out of bounds: %s (content length: %d)", rangeHeader, len(content))
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[start : end+1])
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

func TestClientOpenDirectStream(t *testing.T) {
	t.Parallel()

	content := []byte("0123456789abcdef")
	srv := newDirectAudioServer(t, content)
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestClientOpenCustomHeaders(t *testing.T) {
	t.Parallel()

	content := []byte("hello world")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != "secret" {
			t.Errorf("%s request missing X-Auth header", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			var start, end int
			fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if end >= len(content) {
				end = len(content) - 1
			}
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[start : end+1])
		}
	}))
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{})
	reader, err := client.Open(context.Background(), srv.URL, map[string]string{"X-Auth": "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestClientOpenHEADValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		handler  http.HandlerFunc
		url      string
		noServer bool
		wantErr  string
	}{
		{
			name: "head_non_200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr: "unexpected status 403",
		},
		{
			name: "no_accept_ranges",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.Header().Set("Content-Type", "audio/mpeg")
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "server does not accept \"bytes\" range requests",
		},
		{
			name: "accept_ranges_not_bytes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "none")
				w.Header().Set("Content-Length", "100")
				w.Header().Set("Content-Type", "audio/mpeg")
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "server does not accept \"bytes\" range requests",
		},
		{
			name: "missing_content_length",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Type", "audio/mpeg")
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "valid content length",
		},
		{
			name: "zero_content_length",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "0")
				w.Header().Set("Content-Type", "audio/mpeg")
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "valid content length",
		},
		{
			name:     "invalid_url",
			noServer: true,
			url:      "://bad",
			wantErr:  "HEAD request creation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := tt.url
			if !tt.noServer {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()
				url = srv.URL
			}

			client := httpaudio.NewClient(httpaudio.ClientOptions{})
			reader, err := client.Open(context.Background(), url, nil)
			if err == nil {
				reader.Close()
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
			if reader != nil {
				t.Fatal("expected nil reader on error")
			}
		})
	}
}

func TestClientOpenM3U8Unsupported(t *testing.T) {
	t.Parallel()

	contentTypes := []string{
		"application/vnd.apple.mpegurl",
		"application/x-mpegurl",
		"audio/mpegurl",
		"application/vnd.apple.mpegurl; charset=utf-8",
	}

	for _, ct := range contentTypes {
		t.Run(ct, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ct)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := httpaudio.NewClient(httpaudio.ClientOptions{})
			reader, err := client.Open(context.Background(), srv.URL, nil)
			if err == nil {
				reader.Close()
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, httpaudio.ErrM3U8NotSupported) {
				t.Fatalf("expected ErrM3U8NotSupported, got: %v", err)
			}
		})
	}
}

func TestReadMultipleChunks(t *testing.T) {
	t.Parallel()

	content := []byte("abcdefghijklmnopqrst") // 20 bytes
	var mu sync.Mutex
	var rangeHeaders []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			mu.Lock()
			rangeHeaders = append(rangeHeaders, rangeHeader)
			mu.Unlock()

			var start, end int
			fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if end >= len(content) {
				end = len(content) - 1
			}
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[start : end+1])
		}
	}))
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{ChunkSize: 8})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}

	expectedRanges := []string{"bytes=0-7", "bytes=8-15", "bytes=16-19"}
	mu.Lock()
	defer mu.Unlock()
	if len(rangeHeaders) != len(expectedRanges) {
		t.Fatalf("expected %d range requests, got %d: %v", len(expectedRanges), len(rangeHeaders), rangeHeaders)
	}
	for i, want := range expectedRanges {
		if rangeHeaders[i] != want {
			t.Errorf("range request %d: got %q, want %q", i, rangeHeaders[i], want)
		}
	}
}

func TestReadSmallBuffer(t *testing.T) {
	t.Parallel()

	content := []byte("0123456789")
	srv := newDirectAudioServer(t, content)
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{ChunkSize: 10})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	var result []byte
	buf := make([]byte, 3)
	for {
		n, err := reader.Read(buf)
		result = append(result, buf[:n]...)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if !bytes.Equal(result, content) {
		t.Fatalf("content mismatch: got %q, want %q", result, content)
	}
}

func TestReadAfterClose(t *testing.T) {
	t.Parallel()

	content := []byte("hello")
	srv := newDirectAudioServer(t, content)
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader.Close()

	buf := make([]byte, 10)
	n, err := reader.Read(buf)
	if n != 0 {
		t.Fatalf("expected 0 bytes read, got %d", n)
	}
	if err == nil {
		t.Fatal("expected error after reading closed reader")
	}
	if !strings.Contains(err.Error(), "reader is closed") {
		t.Fatalf("expected error containing %q, got: %v", "reader is closed", err)
	}
}

func TestCloseReturnsNil(t *testing.T) {
	t.Parallel()

	content := []byte("hello")
	srv := newDirectAudioServer(t, content)
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("expected nil error from Close, got: %v", err)
	}
}

func TestReadChunkFetchError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		getStatus  int
		wantErrMsg string
	}{
		{"server_500", http.StatusInternalServerError, "unexpected status 500"},
		{"server_404", http.StatusNotFound, "unexpected status 404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("Content-Length", "100")
					w.Header().Set("Content-Type", "audio/mpeg")
					w.WriteHeader(http.StatusOK)
				case http.MethodGet:
					w.WriteHeader(tt.getStatus)
				}
			}))
			defer srv.Close()

			client := httpaudio.NewClient(httpaudio.ClientOptions{})
			reader, err := client.Open(context.Background(), srv.URL, nil)
			if err != nil {
				t.Fatalf("unexpected error creating reader: %v", err)
			}
			defer reader.Close()

			buf := make([]byte, 10)
			_, err = reader.Read(buf)
			if err == nil {
				t.Fatal("expected error on Read, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrMsg, err)
			}
		})
	}
}

func TestReadCustomHeadersOnChunks(t *testing.T) {
	t.Parallel()

	content := []byte("hello")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != "secret" {
			t.Errorf("%s request missing X-Auth header", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			var start, end int
			fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if end >= len(content) {
				end = len(content) - 1
			}
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[start : end+1])
		}
	}))
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{ChunkSize: 32})
	reader, err := client.Open(context.Background(), srv.URL, map[string]string{"X-Auth": "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error reading: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()

	content := []byte("abc")
	srv := newDirectAudioServer(t, content)
	defer srv.Close()

	client := httpaudio.NewClient(httpaudio.ClientOptions{})
	reader, err := client.Open(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reader.Close()
}
