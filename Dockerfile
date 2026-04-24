# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.2
ARG LIBDAVE_VERSION=1.1.0
ARG YTDLP_VERSION=2026.03.17
ARG DENO_VERSION=2.7.13

FROM debian:trixie-slim AS libdavefetcher
ARG LIBDAVE_VERSION
RUN apt-get update \
       && apt-get install -y --no-install-recommends curl unzip ca-certificates \
       && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /out/usr/local/include /out/usr/local/lib/pkgconfig \
       && curl -fsSL "https://github.com/discord/libdave/releases/download/v${LIBDAVE_VERSION}/cpp/libdave-Linux-X64-boringssl.zip" -o /tmp/libdave.zip \
       && unzip -j /tmp/libdave.zip "include/dave/dave.h" -d /out/usr/local/include \
       && unzip -j /tmp/libdave.zip "lib/libdave.so"      -d /out/usr/local/lib \
       && rm /tmp/libdave.zip \
       && printf '%s\n' \
       'prefix=/usr/local' \
       'exec_prefix=${prefix}' \
       'libdir=${exec_prefix}/lib' \
       'includedir=${prefix}/include' \
       '' \
       'Name: dave' \
       'Description: Discord Audio & Video End-to-End Encryption (DAVE) Protocol' \
       "Version: ${LIBDAVE_VERSION}" \
       'URL: https://github.com/discord/libdave' \
       'Libs: -L${libdir} -ldave' \
       'Cflags: -I${includedir}' \
       > /out/usr/local/lib/pkgconfig/dave.pc


FROM golang:1.26.2-trixie AS builder
RUN apt-get update \
       && apt-get install -y --no-install-recommends pkg-config \
       && rm -rf /var/lib/apt/lists/*
COPY --from=libdavefetcher /out/ /
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/rythm5 ./cmd/app


FROM debian:trixie-slim AS runtime
ARG YTDLP_VERSION
ARG DENO_VERSION

RUN apt-get update \
       && apt-get install -y --no-install-recommends \
       ffmpeg ca-certificates curl unzip \
       && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://github.com/yt-dlp/yt-dlp/releases/download/${YTDLP_VERSION}/yt-dlp_linux.zip" -o /tmp/ytdlp.zip \
       && mkdir -p /opt/yt-dlp \
       && unzip -q /tmp/ytdlp.zip -d /opt/yt-dlp \
       && mv /opt/yt-dlp/yt-dlp_linux /opt/yt-dlp/yt-dlp \
       && chmod +x /opt/yt-dlp/yt-dlp \
       && ln -s /opt/yt-dlp/yt-dlp /usr/local/bin/yt-dlp \
       && rm /tmp/ytdlp.zip

RUN curl -fsSL "https://github.com/denoland/deno/releases/download/v${DENO_VERSION}/deno-x86_64-unknown-linux-gnu.zip" -o /tmp/deno.zip \
       && unzip -q /tmp/deno.zip -d /usr/local/bin \
       && chmod +x /usr/local/bin/deno \
       && rm /tmp/deno.zip

COPY --from=libdavefetcher /out/usr/local/lib/libdave.so /usr/local/lib/libdave.so
RUN ldconfig

RUN useradd --system --uid 10001 --home-dir /app --create-home rythm5
WORKDIR /app
COPY --from=builder --chown=rythm5:rythm5 /out/rythm5 /app/rythm5

USER rythm5
CMD ["/app/rythm5", "-config", "rythm5.toml"]
