# Finch

cliamp plays the music of your own Finch server.

## Setup

Add the server address to `~/.config/cliamp/config.toml`:

```toml
[finch]
url = "https://finch.example.com"
```

The provider starts when `url` is set. Finch needs no credentials.

Open it at start with `provider = "finch"` in the config, or with
`cliamp --provider finch`. Press `F` to switch to it while cliamp runs.

## What you can play

The provider pane lists two sections:

- **Albums**: every album of the server, as `Artist - Album`.
- **Playlists**: the playlists of the server, then `DJ mixes` and `Tracks`.
  `Tracks` holds the tracks that belong to no album.

Finch tracks are finite files. cliamp buffers them, so seeking and gapless
playback work.

## Radio

Set `radio_url` to the address of the Finch radio:

```toml
[finch]
url = "http://192.168.0.250:25520"
radio_url = "http://192.168.0.250:25521"
```

When you play a stream from that host and port, such as its HLS playlist,
cliamp polls `radio_url` + `/api/now-playing` every 5 seconds and shows the
track as `Artist - Title`. When no track plays, it shows the show title. A new
title appears at `started_at` plus `delay_seconds`, when the listener hears
the track. `radio_url` works without `url`.

## Limits

- There is no search. `Ctrl+F` searches YouTube while Finch is active.
- `cliamp setup` does not configure Finch. Edit the config file.
