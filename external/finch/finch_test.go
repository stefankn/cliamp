package finch

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bjarneo/cliamp/config"
	"github.com/bjarneo/cliamp/playlist"
)

// newTestServer serves a small Finch library. It answers 404 for any other
// path, and for a path with a doubled slash.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	const mix = `{"items":[{"id":7,"title":"Mix","artist":"DJ","length":3600.5,"genre":"House"}]}`
	const single = `{"items":[{"id":8,"title":"Single","artist":"Solo","length":200}]}`
	routes := map[string]string{
		"/api/v1/albums?per=10000":                     `{"items":[{"id":1,"title":"Album","artist":"Artist"}]}`,
		"/api/v1/playlists":                            `[{"id":2,"name":"Favorites"}]`,
		"/api/v1/albums/1/items":                       `[{"id":5,"track":3,"disc":2,"title":"Song","artist":"Artist","album":"Album","length":181.7,"genre":"Rock"}]`,
		"/api/v1/playlists/2/entries":                  `[{"id":9,"index":4,"playlist_name":"Favorites","item":{"id":6,"title":"Entry","artist":"Other","album":"Elsewhere","length":60}}]`,
		"/api/v1/items?filter=dj-mix&per=10000":        mix,
		"/api/v1/items?filter=uncategorized&per=10000": single,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.RequestURI()]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewFromConfig(t *testing.T) {
	if p := NewFromConfig(config.FinchConfig{}); p != nil {
		t.Fatalf("NewFromConfig with no URL = %v, want nil", p)
	}
	p := NewFromConfig(config.FinchConfig{URL: "https://finch.example.com/"})
	if p == nil || p.baseURL != "https://finch.example.com" {
		t.Fatalf("NewFromConfig = %+v, want the URL without its trailing slash", p)
	}
	if p.Name() != "Finch" {
		t.Fatalf("Name() = %q, want Finch", p.Name())
	}
}

func TestPlaylists(t *testing.T) {
	srv := newTestServer(t)
	p := NewFromConfig(config.FinchConfig{URL: srv.URL})

	got, err := p.Playlists()
	if err != nil {
		t.Fatalf("Playlists: %v", err)
	}
	want := []playlist.PlaylistInfo{
		{ID: "album:1", Name: "Artist - Album", Section: "Albums"},
		{ID: "playlist:2", Name: "Favorites", Section: "Playlists"},
		{ID: "djmixes", Name: "DJ mixes", Section: "Playlists"},
		{ID: "tracks", Name: "Tracks", Section: "Playlists"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Playlists() = %+v, want %+v", got, want)
	}
}

func TestTracks(t *testing.T) {
	srv := newTestServer(t)
	stream := func(id string) string { return srv.URL + "/api/v1/items/" + id + "/stream" }

	tests := []struct {
		name string
		url  string // the configured server URL
		id   string
		want playlist.Track
	}{
		{
			name: "album",
			url:  srv.URL,
			id:   "album:1",
			want: playlist.Track{Path: stream("5"), TrackNumber: 203, Title: "Song", Artist: "Artist", Album: "Album", Genre: "Rock", DurationSecs: 181, Stream: true},
		},
		{
			name: "playlist",
			url:  srv.URL,
			id:   "playlist:2",
			want: playlist.Track{Path: stream("6"), TrackNumber: 4, Title: "Entry", Artist: "Other", Album: "Favorites", DurationSecs: 60, Stream: true},
		},
		{
			name: "dj mixes",
			url:  srv.URL,
			id:   "djmixes",
			want: playlist.Track{Path: stream("7"), Title: "Mix", Artist: "DJ", Album: "DJ mix", Genre: "House", DurationSecs: 3600, Stream: true},
		},
		{
			name: "single tracks",
			url:  srv.URL,
			id:   "tracks",
			want: playlist.Track{Path: stream("8"), Title: "Single", Artist: "Solo", Album: "Tracks", DurationSecs: 200, Stream: true},
		},
		{
			name: "server URL with a trailing slash",
			url:  srv.URL + "/",
			id:   "tracks",
			want: playlist.Track{Path: stream("8"), Title: "Single", Artist: "Solo", Album: "Tracks", DurationSecs: 200, Stream: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewFromConfig(config.FinchConfig{URL: tt.url})
			got, err := p.Tracks(tt.id)
			if err != nil {
				t.Fatalf("Tracks(%q): %v", tt.id, err)
			}
			if len(got) != 1 {
				t.Fatalf("Tracks(%q) returned %d tracks, want 1", tt.id, len(got))
			}
			g := got[0]
			if g.Path != tt.want.Path || g.TrackNumber != tt.want.TrackNumber || g.Title != tt.want.Title ||
				g.Artist != tt.want.Artist || g.Album != tt.want.Album || g.Genre != tt.want.Genre ||
				g.DurationSecs != tt.want.DurationSecs || g.Stream != tt.want.Stream || g.Realtime {
				t.Fatalf("Tracks(%q) = %+v, want %+v", tt.id, g, tt.want)
			}
			if !IsStreamURL(g.Path) {
				t.Fatalf("IsStreamURL(%q) = false, want true", g.Path)
			}
		})
	}
}

func TestTracksErrors(t *testing.T) {
	srv := newTestServer(t)
	p := NewFromConfig(config.FinchConfig{URL: srv.URL})

	tests := []struct {
		name string
		id   string
		want string // a part of the error
	}{
		{name: "unknown id", id: "artist:1", want: "unknown id format"},
		{name: "missing album", id: "album:99", want: "unexpected status 404"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Tracks(tt.id)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Tracks(%q) error = %v, want one with %q", tt.id, err, tt.want)
			}
		})
	}
}

func TestIsStreamURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://finch.example.com/api/v1/items/12/stream", true},
		{"http://192.168.1.5:8080/music/api/v1/items/12/stream", true},
		{"https://finch.example.com/api/v1/items/12", false},
		{"https://finch.example.com/api/v1/albums/12/items", false},
		{"https://radio.example.com/stream", false},
		{"https://abs.example.com/api/items/12/file/3", false},
		{"/music/song.mp3", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := IsStreamURL(tt.url); got != tt.want {
				t.Fatalf("IsStreamURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
