package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/szczursonn/rythm5/internal/proclimit"
)

const errPrefix = "ytdlp: "

type Client struct {
	binaryPath     string
	cookieFilePath string
	baseArgs       []string
	semaphore      chan struct{}
}

type ClientOptions struct {
	BinaryPath     string
	CookieFilePath string
	CacheEnabled   bool
	CacheDir       string
	MaxConcurrency int
}

func NewClient(opts ClientOptions) *Client {
	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		binaryPath = "yt-dlp"
	}

	baseArgs := []string{"-J", "--flat-playlist"}
	if !opts.CacheEnabled {
		baseArgs = append(baseArgs, "--no-cache-dir")
	} else if opts.CacheDir != "" {
		baseArgs = append(baseArgs, "--cache-dir", opts.CacheDir)
	}

	if opts.CookieFilePath != "" {
		baseArgs = append(baseArgs, "--cookies", opts.CookieFilePath)
	}

	return &Client{
		binaryPath:     binaryPath,
		cookieFilePath: opts.CookieFilePath,
		baseArgs:       baseArgs,
		semaphore:      make(chan struct{}, max(opts.MaxConcurrency, 1)),
	}
}

type URLPreference int

const (
	URLPreferenceNone URLPreference = iota
	URLPreferenceSingle
	URLPreferenceCollection
)

func (client *Client) GetAudioResourcesByURL(ctx context.Context, url string, preferPlaylists bool) (URLQueryResult, error) {
	playlistPreferenceArg := "--no-playlist"
	if preferPlaylists {
		playlistPreferenceArg = "--yes-playlist"
	}

	res, err := client.callYtdlp(ctx, playlistPreferenceArg, url)
	if err != nil {
		return URLQueryResult{}, err
	}

	if len(res.Entries) > 0 {
		return URLQueryResult{
			PlaylistInfo: res.extrackAudioResourcePlaylistInfo(),
		}, nil
	}

	info, err := res.extrackAudioResourceInfo()
	if err != nil {
		return URLQueryResult{}, err
	}

	return URLQueryResult{
		SingleInfo: info,
	}, nil
}

func (client *Client) GetAudioResourcesByYoutubeSearch(ctx context.Context, query string, maxResults int) ([]*AudioResourceInfo, error) {
	res, err := client.callYtdlp(ctx, fmt.Sprintf("ytsearch%d:%s", max(maxResults, 1), query))
	if err != nil {
		return nil, err
	}

	return res.extrackAudioResourcePlaylistInfo().Entries, nil
}

func (client *Client) callYtdlp(ctx context.Context, args ...string) (*ytdlpResult, error) {
	select {
	case client.semaphore <- struct{}{}:
		defer func() {
			<-client.semaphore
		}()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	cmdArgs := make([]string, 0, len(client.baseArgs)+len(args))
	cmdArgs = append(cmdArgs, client.baseArgs...)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, client.binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to start process: %w", err)
	}

	proclimit.ApplyOOMKillerPriority(cmd.Process.Pid, proclimit.OOMKillerPriorityHigh)
	proclimit.ApplyCPUPriority(cmd.Process.Pid, proclimit.CPUPriorityLow)

	if err := cmd.Wait(); err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
		return nil, err
	}

	res := &ytdlpResult{}
	if err := json.Unmarshal(stdout.Bytes(), res); err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to parse output (stderr: %s): %w", stderr.String(), err)
	}

	if res.Title == "" {
		return nil, fmt.Errorf(errPrefix+"empty title in result json (stderr: %s)", stderr.String())
	}

	return res, nil
}
