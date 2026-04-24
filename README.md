# rythm5 - Discord Music Bot

![play command invocation](/docs/query.png)

- Supports Youtube, Soundcloud and Spotify
- Requires [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org/)
- Build: Requires CGO (depends on [godave](https://github.com/disgoorg/godave))
- Supersedes [rythm4](https://github.com/szczursonn/rythm4)

## Minimal [rythm5.toml](/rythm5.example.toml) config file

```toml
# Bot token from the Discord Developer Portal.
discord_token = "xyz"
```

## Recommended options for small VMs

```toml
[transcoder]
cpu_priority = "low"
oom_killer_priority = "above_normal"

[ytdlp]
cpu_priority = "low"
oom_killer_priority = "high"
```

## Docker

### Build

```sh
docker build -t rythm5 .
```

### Run

```sh
docker run --rm \
    -v "$PWD/rythm5.toml:/rythm5.toml:ro" \
    rythm5
```

### yt-dlp cookie file

```sh
-v "$PWD/cookies.txt:/app/cookies.txt"
```

```toml
[ytdlp]
cookie_path = "/app/cookies.txt"
```

### Log file

```sh
-v "$PWD/rythm5.jsonl:/app/rythm5.jsonl"
```
