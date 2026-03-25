package ytdlp

import (
	"fmt"
	"time"
)

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

type AudioResourceInfo struct {
	Title             string
	Duration          time.Duration
	WebpageURL        string
	StreamURL         string
	StreamHTTPHeaders map[string]string
}

type AudioResourcePlaylistInfo struct {
	Title      string
	WebpageURL string
	Entries    []*AudioResourceInfo
}

type URLQueryResult struct {
	SingleInfo   *AudioResourceInfo
	PlaylistInfo *AudioResourcePlaylistInfo
}

func (yr *ytdlpResult) extrackAudioResourcePlaylistInfo() *AudioResourcePlaylistInfo {
	info := &AudioResourcePlaylistInfo{
		Title:      yr.Title,
		WebpageURL: yr.WebpageURL,
		Entries:    make([]*AudioResourceInfo, 0, len(yr.Entries)),
	}

	for _, entry := range yr.Entries {
		audioResourceInfo, err := entry.extrackAudioResourceInfo()
		if err == nil {
			info.Entries = append(info.Entries, audioResourceInfo)
		}
	}

	return info
}

func (yre *ytdlpResultEntry) extrackAudioResourceInfo() (*AudioResourceInfo, error) {
	if yre.Title == "" {
		return nil, fmt.Errorf(errPrefix + "empty title")
	}

	info := &AudioResourceInfo{
		Title:    yre.Title,
		Duration: time.Second * time.Duration(yre.Duration),
	}

	for _, format := range yre.Formats {
		if (format.Protocol == "http" || format.Protocol == "https") && format.URL != "" && format.ACodec != "none" {
			info.StreamURL = format.URL
			info.StreamHTTPHeaders = format.HTTPHeaders
			break
		}
	}

	if yre.WebpageURL == "" {
		info.WebpageURL = yre.URL
	} else {
		info.WebpageURL = yre.WebpageURL
	}

	return info, nil
}
