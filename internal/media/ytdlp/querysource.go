package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"

	"github.com/szczursonn/rythm5/internal/httpaudio"
	"github.com/szczursonn/rythm5/internal/media"
	"github.com/szczursonn/rythm5/internal/proclimit"
)

const errPrefix = "media/ytdlp: "

type querySource struct {
	httpAudio         *httpaudio.Client
	binaryPath        string
	cookieFilePath    string
	baseArgs          []string
	cpuPriority       proclimit.CPUPriority
	oomKillerPriority proclimit.OOMKillerPriority

	semaphore chan struct{}
}

var _ media.QuerySource = (*querySource)(nil)

type QuerySourceOptions struct {
	HttpAudio         *httpaudio.Client
	BinaryPath        string
	CookieFilePath    string
	CacheEnabled      bool
	CacheDir          string
	MaxConcurrency    int
	CPUPriority       proclimit.CPUPriority
	OOMKillerPriority proclimit.OOMKillerPriority
}

func NewQuerySource(opts QuerySourceOptions) media.QuerySource {
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

	return &querySource{
		httpAudio:         opts.HttpAudio,
		binaryPath:        binaryPath,
		cookieFilePath:    opts.CookieFilePath,
		baseArgs:          baseArgs,
		cpuPriority:       opts.CPUPriority,
		oomKillerPriority: opts.OOMKillerPriority,
		semaphore:         make(chan struct{}, max(opts.MaxConcurrency, 1)),
	}
}

type ytdlpResult struct {
	ytdlpResultEntry
	Entries []ytdlpResultEntry `json:"entries"`
}

type ytdlpResultEntry struct {
	Title      string                   `json:"title"`
	Duration   float64                  `json:"duration"`
	WebpageURL string                   `json:"webpage_url"`
	URL        string                   `json:"url"`
	Formats    []ytdlpResultEntryFormat `json:"formats"`
}

type ytdlpResultEntryFormat struct {
	Protocol    string            `json:"protocol"`
	URL         string            `json:"url"`
	HTTPHeaders map[string]string `json:"http_headers"`
	ACodec      string            `json:"acodec"`
}

const (
	schemeYoutubeSearch      = "ytsearch"
	schemeYoutubeMusicSearch = "ytmsearch"
	schemeSoundcloudSearch   = "scsearch"
)

func (qs *querySource) SupportedPrefixSchemes() []string {
	return []string{
		media.QuerySchemeGenericSearch,
		schemeYoutubeSearch,
		schemeYoutubeMusicSearch,
		schemeSoundcloudSearch,
	}
}

func (qs *querySource) Query(ctx context.Context, query *url.URL) (media.QueryResult, error) {
	switch query.Scheme {
	case "http", "https":
		return qs.queryHTTP(ctx, query)
	case media.QuerySchemeGenericSearch, schemeYoutubeSearch:
		return qs.querySearch(ctx, "ytsearch", query.Path)
	case schemeYoutubeMusicSearch:
		return qs.querySearch(ctx, "ytmsearch", query.Path)
	case schemeSoundcloudSearch:
		return qs.querySearch(ctx, "scsearch", query.Path)
	default:
		return media.QueryResult{}, media.ErrUnsupportedQuery
	}
}

func (qs *querySource) querySearch(ctx context.Context, ytdlpPrefix string, query string) (media.QueryResult, error) {
	if query == "" {
		return media.QueryResult{}, media.ErrUnsupportedQuery
	}

	res, err := qs.execYtDlp(ctx, fmt.Sprintf("%s1:%s", ytdlpPrefix, query))
	if err != nil {
		return media.QueryResult{}, err
	}

	tracks := qs.extractPlaylist(res).Tracks()
	if len(tracks) == 0 {
		return media.QueryResult{}, fmt.Errorf(errPrefix + "search returned 0 results")
	}

	return media.QueryResult{
		Track: tracks[0],
	}, nil
}

func (qs *querySource) queryHTTP(ctx context.Context, query *url.URL) (media.QueryResult, error) {
	switch query.Host {
	case "www.youtube.com", "youtube.com", "music.youtube.com", "www.youtu.be", "youtu.be", "soundcloud.com", "on.soundcloud.com":
		break
	default:
		return media.QueryResult{}, media.ErrUnsupportedQuery
	}

	res, err := qs.execYtDlp(ctx, "--no-playlist", query.String())
	if err != nil {
		return media.QueryResult{}, err
	}

	if len(res.Entries) > 0 {
		return media.QueryResult{
			Playlist: qs.extractPlaylist(res),
		}, nil
	}

	t, err := qs.extractTrack(&res.ytdlpResultEntry)
	if err != nil {
		return media.QueryResult{}, err
	}

	return media.QueryResult{
		Track: t,
	}, nil
}

func (qs *querySource) execYtDlp(ctx context.Context, args ...string) (*ytdlpResult, error) {
	select {
	case qs.semaphore <- struct{}{}:
		defer func() {
			<-qs.semaphore
		}()
	case <-ctx.Done():
		return nil, fmt.Errorf(errPrefix+"waiting on semaphore: %w", ctx.Err())
	}

	cmdArgs := make([]string, 0, len(qs.baseArgs)+len(args))
	cmdArgs = append(cmdArgs, qs.baseArgs...)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, qs.binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to start process: %w", err)
	}

	if qs.oomKillerPriority != proclimit.OOMKillerPriorityUnset {
		proclimit.ApplyOOMKillerPriority(cmd.Process.Pid, qs.oomKillerPriority)
	}
	if qs.cpuPriority != proclimit.CPUPriorityUnset {
		proclimit.ApplyCPUPriority(cmd.Process.Pid, qs.cpuPriority)
	}

	// no error check: yt-dlp sometimes returns non-zero status if error occured, even if it has successfully recovered from it
	cmd.Wait()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(errPrefix+"running yt-dlp: %w", err)
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
