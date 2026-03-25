package rangeread_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/szczursonn/rythm5/internal/rangeread"
)

// ⚠️⚠️⚠️ AI-generated ⚠️⚠️⚠️

func newTestServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
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

func TestNewRangeReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		handler  http.HandlerFunc
		url      string // if set, overrides server URL
		wantErr  string
		noServer bool
	}{
		{
			name: "success",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
			}),
		},
		{
			name: "head_non_200",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			}),
			wantErr: "unexpected status 403",
		},
		{
			name: "no_accept_ranges",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
			}),
			wantErr: "does not support bytes range requests",
		},
		{
			name: "accept_ranges_not_bytes",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "none")
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
			}),
			wantErr: "does not support bytes range requests",
		},
		{
			name: "missing_content_length",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.WriteHeader(http.StatusOK)
			}),
			wantErr: "valid content length",
		},
		{
			name: "zero_content_length",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(http.StatusOK)
			}),
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

			reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
				URL: url,
			})

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				if reader != nil {
					t.Fatal("expected nil reader on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if reader == nil {
				t.Fatal("expected non-nil reader")
			}
			reader.Close()
		})
	}
}

func TestNewRangeReaderDefaults(t *testing.T) {
	t.Parallel()

	content := []byte("hello")
	srv := newTestServer(t, content)
	defer srv.Close()

	// All zero-value options except URL — defaults should be applied internally.
	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()
}

func TestNewRangeReaderCustomHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "testvalue" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL:         srv.URL,
		HTTPHeaders: map[string]string{"X-Custom": "testvalue"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()
}

func TestReadSingleChunk(t *testing.T) {
	t.Parallel()

	content := []byte("0123456789")
	srv := newTestServer(t, content)
	defer srv.Close()

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL:       srv.URL,
		ChunkSize: 32,
	})
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

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL:       srv.URL,
		ChunkSize: 8,
	})
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

	// Verify Range headers
	expectedRanges := []string{
		"bytes=0-7",
		"bytes=8-15",
		"bytes=16-19",
	}
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

	content := []byte("0123456789") // 10 bytes
	srv := newTestServer(t, content)
	defer srv.Close()

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL:       srv.URL,
		ChunkSize: 10,
	})
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
	srv := newTestServer(t, content)
	defer srv.Close()

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL: srv.URL,
	})
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
	srv := newTestServer(t, content)
	defer srv.Close()

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL: srv.URL,
	})
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
		{
			name:       "server_500",
			getStatus:  http.StatusInternalServerError,
			wantErrMsg: "unexpected status 500",
		},
		{
			name:       "server_404",
			getStatus:  http.StatusNotFound,
			wantErrMsg: "unexpected status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("Content-Length", "100")
					w.WriteHeader(http.StatusOK)
				case http.MethodGet:
					w.WriteHeader(tt.getStatus)
				}
			}))
			defer srv.Close()

			reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
				URL: srv.URL,
			})
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

	reader, err := rangeread.NewRangeReader(context.Background(), rangeread.RangeReaderOptions{
		URL:         srv.URL,
		HTTPHeaders: map[string]string{"X-Auth": "secret"},
		ChunkSize:   32,
	})
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
