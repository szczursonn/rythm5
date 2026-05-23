# rythm5 - Discord Music Bot

![play command invocation](/docs/query.png)

- Supports Youtube, Soundcloud and Spotify audio playback
- Depends on [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org/)
- Requires CGO (depends on [godave](https://github.com/disgoorg/godave))
- Supersedes [rythm4](https://github.com/szczursonn/rythm4)

## Minimal [rythm5.toml](/rythm5.example.toml) config file

```toml
# Bot token from the Discord Developer Portal.
discord_token = "xyz"
```

## Recommended options for small VMs

To keep them from exploding

```toml
[transcoder]
nice_value = 5
oom_score_adj = 500

[ytdlp]
nice_value = 10
oom_score_adj = 1000
```

## Docker

The container expects `/data` to be mounted.

### Build

```sh
docker build -t rythm5 .
```

### Run

```sh
docker run -d --init --restart unless-stopped \
    -v "$PWD/data:/data" \
    rythm5
```

## TODO

- Get rid of CGO
