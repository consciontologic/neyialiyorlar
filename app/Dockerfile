# Flutter build container for neyialiyorlar PWA
# Builds the Flutter web app and outputs to /app/build/web

FROM debian:bookworm-slim as build-env

# Install Flutter dependencies
RUN apt-get update && apt-get install -y \
  curl \
  git \
  unzip \
  xz-utils \
  lib32stdc++6 \
  lib32z1 \
  libgl1-mesa-glx \
  && rm -rf /var/lib/apt/lists/*

# Download and install Flutter (pinned version)
ARG FLUTTER_VERSION=3.22.0
RUN git clone --depth 1 --branch ${FLUTTER_VERSION} https://github.com/flutter/flutter.git /flutter

ENV PATH="/flutter/bin:/flutter/bin/cache/dart-sdk/bin:${PATH}"

# Pre-cache Dart pub
RUN flutter precache --web

WORKDIR /app
COPY app/pubspec.yaml app/pubspec.lock* ./
RUN flutter pub get

# Copy app source (CACHEBUST invalidates this layer + flutter build below on each make app.build)
ARG CACHEBUST=1
COPY app/ .

# Build web app with optimization
# --web-renderer html: uses DOM/CSS instead of WebGL (CanvasKit).
# Better Firefox compatibility, lower GPU/CPU usage, no WebAssembly for rendering.
RUN flutter build web \
  --release \
  --web-renderer html \
  --tree-shake-icons

# Export stage: copy the built bundle into the mounted flutter_build volume.
# The main nginx service (see docker-compose.yml) serves that volume; this
# image is a one-shot populator, run via `docker compose run --rm flutter-build`.
FROM alpine:3.20 AS export

COPY --from=build-env /app/build/web /opt/web

# The flutter_build volume is mounted at /usr/share/nginx/html at run time.
CMD ["sh", "-c", "rm -rf /usr/share/nginx/html/* /usr/share/nginx/html/.[!.]* 2>/dev/null; cp -a /opt/web/. /usr/share/nginx/html/ && echo 'flutter web bundle copied into volume'"]
