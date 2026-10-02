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
`cliamp --provider finch`.

## What you can play

The provider pane lists two sections:

- **Albums**: every album of the server, as `Artist - Album`.
- **Playlists**: the playlists of the server, then `DJ mixes` and `Tracks`.
  `Tracks` holds the tracks that belong to no album.

Finch tracks are finite files. cliamp buffers them, so seeking and gapless
playback work.

## Limits

- There is no search. `Ctrl+F` searches YouTube while Finch is active.
- `cliamp setup` does not configure Finch. Edit the config file.
