package model

import "cliamp/playlist"

func (m Model) currentPlaybackTrack() (playlist.Track, int) {
	if m.playingTrackActive && (m.buffering || (m.player != nil && m.player.IsPlaying())) {
		return m.playingTrack, 0
	}
	if m.playlist == nil {
		return playlist.Track{}, -1
	}
	return m.playlist.Current()
}

func (m *Model) setPlaybackTrack(track playlist.Track) {
	m.playingTrack = track
	m.playingTrackActive = true
	m.playbackDetached = false
}

func (m *Model) detachPlaybackTrack() {
	track, idx := m.currentPlaybackTrack()
	if idx < 0 {
		return
	}
	m.playingTrack = track
	m.playingTrackActive = true
	m.playbackDetached = true
}

func (m *Model) clearPlaybackTrack() {
	m.playingTrack = playlist.Track{}
	m.playingTrackActive = false
	m.playbackDetached = false
}
