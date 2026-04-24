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
