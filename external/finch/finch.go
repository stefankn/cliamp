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
	default:
		return nil, fmt.Errorf("finch: unknown id format: %s", id)
	}

	return result, nil
}

func (p *Provider) fetchAlbums() ([]album, error) {
	u, err := url.Parse(strings.TrimRight(p.cfg.URL, "/") + "/api/v1/albums")
	if err != nil {
		return nil, fmt.Errorf("finch: parse url: %w", err)
	}

	q := u.Query()
	q.Set("per", "10000")
	u.RawQuery = q.Encode()

	resp, err := p.client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("finch: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finch: unexpected status %d from %s", resp.StatusCode, u)
	}

	var result albumResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("finch: decode albums response: %w", err)
	}

	return result.Albums, nil
}

func (p *Provider) fetchItems(albumId string) ([]item, error) {
	u, err := url.Parse(strings.TrimRight(p.cfg.URL, "/") + "/api/v1/albums/" + albumId + "/items")
	if err != nil {
		return nil, fmt.Errorf("finch: parse url: %w", err)
	}

	q := u.Query()
	u.RawQuery = q.Encode()

	resp, err := p.client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("finch: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finch: unexpected status %d from %s", resp.StatusCode, u)
	}

	var result []item
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("finch: decode items response: %w", err)
	}

	return result, nil
}

func (p *Provider) fetchPlaylists() ([]playlistResponse, error) {
	u, err := url.Parse(strings.TrimRight(p.cfg.URL, "/") + "/api/v1/playlists")
	if err != nil {
		return nil, fmt.Errorf("finch: parse url: %w", err)
	}

	q := u.Query()
	u.RawQuery = q.Encode()

	resp, err := p.client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("finch: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finch: unexpected status %d from %s", resp.StatusCode, u)
	}

	var result []playlistResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("finch: decode playlists response: %w", err)
	}

	return result, nil
}

func (p *Provider) fetchPlaylistItems(playlistId string) ([]playlistEntry, error) {
	u, err := url.Parse(strings.TrimRight(p.cfg.URL, "/") + "/api/v1/playlists/" + playlistId + "/entries")
	if err != nil {
		return nil, fmt.Errorf("finch: parse url: %w", err)
	}

	q := u.Query()
	u.RawQuery = q.Encode()

	resp, err := p.client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("finch: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finch: unexpected status %d from %s", resp.StatusCode, u)
	}

	var result []playlistEntry
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("finch: decode playlist entries response: %w", err)
	}

	return result, err
}
