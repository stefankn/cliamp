package finch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bjarneo/cliamp/config"
	"github.com/bjarneo/cliamp/internal/httpclient"
)

// radioPollInterval is how often the radio's now-playing endpoint is polled.
// The server is on the local network, and a short interval keeps the title
// close to the moment the listener hears the track change.
const radioPollInterval = 5 * time.Second

// maxNowPlayingBody limits a now-playing response. It is under a KB.
const maxNowPlayingBody = 1 << 20

type nowPlayingResponse struct {
	Item struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
	} `json:"item"`
	Show struct {
		Title string `json:"title"`
	} `json:"show"`
	StartedAt    string  `json:"started_at"`
	DelaySeconds float64 `json:"delay_seconds"`
}

// NowPlayingResolver returns a player.StreamMetadataResolver for the Finch
// radio at cfg.RadioURL, or nil when no radio is configured. It claims any
// stream on the radio's host and port, and polls its /api/now-playing.
func NowPlayingResolver(cfg config.FinchConfig) func(string) (func(context.Context) (string, error), time.Duration, bool) {
	radio, err := url.Parse(strings.TrimRight(cfg.RadioURL, "/"))
	if cfg.RadioURL == "" || err != nil || radio.Hostname() == "" {
		return nil
	}
	endpoint := radio.String() + "/api/now-playing"
	origin := hostPort(radio)
	client := httpclient.NewAPI(8 * time.Second)

	return func(streamURL string) (func(context.Context) (string, error), time.Duration, bool) {
		u, err := url.Parse(streamURL)
		if err != nil || hostPort(u) != origin {
			return nil, 0, false
		}
		var np nowPlaying
		return func(ctx context.Context) (string, error) {
			r, err := fetchNowPlaying(ctx, client, endpoint)
			if err != nil {
				return "", err
			}
			return np.update(r, time.Now()), nil
		}, radioPollInterval, true
	}
}

// hostPort returns the lowercased host and port of u, with the default port
// of its scheme filled in, so http://h and http://h:80 compare equal.
func hostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	return strings.ToLower(u.Hostname()) + ":" + port
}

func fetchNowPlaying(ctx context.Context, client *http.Client, endpoint string) (nowPlayingResponse, error) {
	var r nowPlayingResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return r, fmt.Errorf("finch: now playing: %w", err)
	}
	req.Header.Set("User-Agent", httpclient.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return r, fmt.Errorf("finch: now playing: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return r, fmt.Errorf("finch: now playing: HTTP %d", resp.StatusCode)
	}
	if err := httpclient.ReadJSON(resp.Body, maxNowPlayingBody, &r); err != nil {
		return r, fmt.Errorf("finch: now playing: %w", err)
	}
	return r, nil
}

// nowPlaying tracks the title the listener hears. The server reports a track
// when it starts, but the stream reaches the listener delay_seconds later, so
// a new title waits until started_at + delay_seconds.
type nowPlaying struct {
	heard string
}

// update returns the title the listener hears at now, given response r.
// It returns "" until the first title is audible.
func (n *nowPlaying) update(r nowPlayingResponse, now time.Time) string {
	title := r.title()
	if title == "" {
		return n.heard
	}
	if start, err := time.Parse(time.RFC3339, r.StartedAt); err == nil {
		delay := time.Duration(r.DelaySeconds * float64(time.Second))
		if now.Before(start.Add(delay)) {
			return n.heard
		}
	}
	n.heard = title
	return title
}

// title returns "Artist - Title", the ICY StreamTitle form the UI splits on.
// It falls back to the show title when no track is playing.
func (r nowPlayingResponse) title() string {
	artist := strings.TrimSpace(r.Item.Artist)
	title := strings.TrimSpace(r.Item.Title)
	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case title != "":
		return title
	case artist != "":
		return artist
	}
	return strings.TrimSpace(r.Show.Title)
}
