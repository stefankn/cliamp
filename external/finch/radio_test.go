package finch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bjarneo/cliamp/config"
)

const sampleNowPlaying = `{
  "item": {"id": 4821, "title": "Windowlicker", "artist": "Aphex Twin", "album": "Windowlicker", "length": 367.84},
  "show": {"id": 3, "title": "Morning Show"},
  "backdrop_id": 12,
  "started_at": "2026-10-31T09:14:02.481+01:00",
  "delay_seconds": 24
}`

func TestNowPlayingResolverMatch(t *testing.T) {
	r := NowPlayingResolver(config.FinchConfig{RadioURL: "http://192.168.0.250:25521/"})
	if r == nil {
		t.Fatal("NowPlayingResolver() = nil")
	}
	tests := []struct {
		stream string
		want   bool
	}{
		{"http://192.168.0.250:25521/hls/live.m3u8", true},
		{"https://192.168.0.250:25521/live.m3u8", true},
		{"http://192.168.0.250:25520/api/v1/items/1/stream", false},
		{"http://192.168.0.251:25521/live.m3u8", false},
		{"http://example.com/live.m3u8", false},
	}
	for _, tt := range tests {
		t.Run(tt.stream, func(t *testing.T) {
			fetch, interval, ok := r(tt.stream)
			if ok != tt.want {
				t.Fatalf("ok = %v, want %v", ok, tt.want)
			}
			if ok && (fetch == nil || interval != radioPollInterval) {
				t.Fatalf("fetch nil = %v, interval = %v", fetch == nil, interval)
			}
		})
	}
}

func TestNowPlayingResolverUnset(t *testing.T) {
	for _, raw := range []string{"", "not a url", "/path/only"} {
		if r := NowPlayingResolver(config.FinchConfig{RadioURL: raw}); r != nil {
			t.Errorf("NowPlayingResolver(%q) != nil", raw)
		}
	}
}

func TestHostPortDefaults(t *testing.T) {
	r := NowPlayingResolver(config.FinchConfig{RadioURL: "http://Radio.local"})
	if _, _, ok := r("http://radio.local:80/live.m3u8"); !ok {
		t.Fatal("explicit default port and case difference did not match")
	}
}

func TestNowPlayingUpdate(t *testing.T) {
	start := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	resp := func(artist, title, show string) nowPlayingResponse {
		var r nowPlayingResponse
		r.Item.Artist, r.Item.Title, r.Show.Title = artist, title, show
		r.StartedAt = start.Format(time.RFC3339)
		r.DelaySeconds = 24
		return r
	}
	tests := []struct {
		name  string
		heard string
		r     nowPlayingResponse
		now   time.Time
		want  string
	}{
		{"audible", "", resp("Aphex Twin", "Windowlicker", ""), start.Add(25 * time.Second), "Aphex Twin - Windowlicker"},
		{"in delay keeps previous", "Old - Song", resp("Aphex Twin", "Windowlicker", ""), start.Add(10 * time.Second), "Old - Song"},
		{"in delay first poll", "", resp("Aphex Twin", "Windowlicker", ""), start.Add(10 * time.Second), ""},
		{"title only", "", resp("", "Windowlicker", ""), start.Add(time.Minute), "Windowlicker"},
		{"show fallback", "", resp("", "", "Morning Show"), start.Add(time.Minute), "Morning Show"},
		{"empty keeps previous", "Old - Song", resp("", "", ""), start.Add(time.Minute), "Old - Song"},
		{"bad timestamp shows at once", "", func() nowPlayingResponse {
			r := resp("A", "B", "")
			r.StartedAt = "yesterday"
			return r
		}(), start, "A - B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := nowPlaying{heard: tt.heard}
			if got := n.update(tt.r, tt.now); got != tt.want {
				t.Fatalf("update() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNowPlayingFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/now-playing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleNowPlaying))
	}))
	defer srv.Close()

	r := NowPlayingResolver(config.FinchConfig{RadioURL: srv.URL})
	fetch, _, ok := r(srv.URL + "/live.m3u8")
	if !ok {
		t.Fatal("stream on the radio origin did not match")
	}
	// started_at is in the past, so the title is audible.
	got, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != "Aphex Twin - Windowlicker" {
		t.Fatalf("fetch = %q, want Aphex Twin - Windowlicker", got)
	}
}

func TestNowPlayingFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := NowPlayingResolver(config.FinchConfig{RadioURL: srv.URL})
	fetch, _, _ := r(srv.URL + "/live.m3u8")
	if _, err := fetch(context.Background()); err == nil {
		t.Fatal("fetch error = nil, want HTTP error")
	}
}
