package finch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cliamp/playlist"
)

// Config holds the values read from [finch] in config.toml
type Config struct {
	URL string
}

// Provider implements provider.Browser
type Provider struct {
	cfg    Config
	client *http.Client
}

// New returns a ready-to-use Finch provider
func New(cfg Config) *Provider {
	return &Provider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type albumResponse struct {
	Albums []album `json:"items"`
}

type itemsResponse struct {
	Items []item `json:"items"`
}

type album struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Year   int    `json:"year"`
	Genre  string `json:"genre"`
}

type item struct {
	ID       int     `json:"id"`
	Track    int     `json:"track"`
	Disc     int     `json:"disc"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Duration float32 `json:"length"`
	Genre    string  `json:"genre"`
}

type playlistResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type playlistEntry struct {
	ID       int    `json:"id"`
	Index    int    `json:"index"`
	Item     item   `json:"item"`
	Playlist string `json:"playlist_name"`
}

func (p *Provider) Name() string {
	return "Finch"
}

func (p *Provider) Playlists() ([]playlist.PlaylistInfo, error) {
	type albumsResult struct {
		albums []album
		err    error
	}
	type playlistsResult struct {
		playlists []playlistResponse
		err       error
	}
	type djMixesResult struct {
		djmixes []item
		err     error
	}

	albumsCh := make(chan albumsResult, 1)
	playlistsCh := make(chan playlistsResult, 1)

	go func() {
		albums, err := p.fetchAlbums()
		albumsCh <- albumsResult{albums, err}
	}()
	go func() {
		playlists, err := p.fetchPlaylists()
		playlistsCh <- playlistsResult{playlists, err}
	}()

	albumsRes := <-albumsCh
	playlistsRes := <-playlistsCh

	if albumsRes.err != nil {
		return nil, albumsRes.err
	}
	if playlistsRes.err != nil {
		return nil, playlistsRes.err
	}

	var result []playlist.PlaylistInfo
	for _, a := range albumsRes.albums {
		result = append(result, playlist.PlaylistInfo{
			ID:      "album:" + strconv.Itoa(a.ID),
			Name:    a.Artist + " - " + a.Title,
			Section: "Albums",
		})
	}
	for _, p := range playlistsRes.playlists {
		result = append(result, playlist.PlaylistInfo{
			ID:      "playlist:" + strconv.Itoa(p.ID),
			Name:    p.Name,
			Section: "Playlists",
		})
	}

	result = append(result, playlist.PlaylistInfo{
		ID:      "djmixes",
		Name:    "DJ mixes",
		Section: "Playlists",
	})

	result = append(result, playlist.PlaylistInfo{
		ID:      "tracks",
		Name:    "Tracks",
		Section: "Playlists",
	})

	return result, nil
}

func (p *Provider) Tracks(id string) ([]playlist.Track, error) {
	var result []playlist.Track

	switch {
	case strings.HasPrefix(id, "album:"):
		items, err := p.fetchItems(strings.TrimPrefix(id, "album:"))
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			result = append(result, playlist.Track{
				Path:         p.cfg.URL + "/api/v1/items/" + strconv.Itoa(i.ID) + "/stream",
				TrackNumber:  i.Disc*100 + i.Track,
				Title:        i.Title,
				Artist:       i.Artist,
				Album:        i.Album,
				Stream:       true, // true for HTTP/HTTPS URLs
				Realtime:     false,
				DurationSecs: int(i.Duration),
				Genre:        i.Genre,
			})
		}
	case strings.HasPrefix(id, "playlist:"):
		entries, err := p.fetchPlaylistItems(strings.TrimPrefix(id, "playlist:"))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			result = append(result, playlist.Track{
				Path:         p.cfg.URL + "/api/v1/items/" + strconv.Itoa(e.Item.ID) + "/stream",
				TrackNumber:  e.Index,
				Title:        e.Item.Title,
				Artist:       e.Item.Artist,
				Album:        e.Playlist,
				Stream:       true, // true for HTTP/HTTPS URLs
				Realtime:     false,
				DurationSecs: int(e.Item.Duration),
				Genre:        e.Item.Genre,
			})
		}
	case strings.HasPrefix(id, "djmixes"):
		items, err := p.fetchDjMixes()
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			result = append(result, playlist.Track{
				Path:         p.cfg.URL + "/api/v1/items/" + strconv.Itoa(i.ID) + "/stream",
				Title:        i.Title,
				Artist:       i.Artist,
				Album:        "DJ mix",
				Stream:       true, // true for HTTP/HTTPS URLs
				Realtime:     false,
				DurationSecs: int(i.Duration),
				Genre:        i.Genre,
			})
		}
	case strings.HasPrefix(id, "tracks"):
		items, err := p.fetchSingletonTracks()
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			result = append(result, playlist.Track{
				Path:         p.cfg.URL + "/api/v1/items/" + strconv.Itoa(i.ID) + "/stream",
				Title:        i.Title,
				Artist:       i.Artist,
				Album:        "Tracks",
				Stream:       true, // true for HTTP/HTTPS URLs
				Realtime:     false,
				DurationSecs: int(i.Duration),
				Genre:        i.Genre,
			})
		}
	default:
		return nil, fmt.Errorf("finch: unknown id format: %s", id)
	}

	return result, nil
}

// get performs a GET request to the given API path, decodes the JSON response
// into T, and returns it. params may be nil if no query parameters are needed.
func get[T any](p *Provider, path string, params url.Values) (T, error) {
	var zero T
	u, err := url.Parse(strings.TrimRight(p.cfg.URL, "/") + path)
	if err != nil {
		return zero, fmt.Errorf("finch: parse url: %w", err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	resp, err := p.client.Get(u.String())
	if err != nil {
		return zero, fmt.Errorf("finch: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("finch: unexpected status %d from %s", resp.StatusCode, u)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("finch: decode response from %s: %w", u, err)
	}
	return result, nil
}

func (p *Provider) fetchAlbums() ([]album, error) {
	r, err := get[albumResponse](p, "/api/v1/albums", url.Values{"per": {"10000"}})
	if err != nil {
		return nil, err
	}
	return r.Albums, nil
}

func (p *Provider) fetchDjMixes() ([]item, error) {
	r, err := get[itemsResponse](p, "/api/v1/items", url.Values{"filter": {"dj-mix"}, "per": {"10000"}})
	if err != nil {
		return nil, err
	}
	return r.Items, nil
}

func (p *Provider) fetchSingletonTracks() ([]item, error) {
	r, err := get[itemsResponse](p, "/api/v1/items", url.Values{"filter": {"uncategorized"}, "per": {"10000"}})
	if err != nil {
		return nil, err
	}
	return r.Items, nil
}

func (p *Provider) fetchItems(albumID string) ([]item, error) {
	return get[[]item](p, "/api/v1/albums/"+albumID+"/items", nil)
}

func (p *Provider) fetchPlaylists() ([]playlistResponse, error) {
	return get[[]playlistResponse](p, "/api/v1/playlists", nil)
}

func (p *Provider) fetchPlaylistItems(playlistID string) ([]playlistEntry, error) {
	return get[[]playlistEntry](p, "/api/v1/playlists/"+playlistID+"/entries", nil)
}
