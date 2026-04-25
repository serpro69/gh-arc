#!/usr/bin/env bash
set -e

TAG="${GITHUB_REF#refs/tags/}"
COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.Version=$TAG"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.GitCommit=$COMMIT"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.BuildDate=$DATE"

platforms=(
  darwin-amd64
  darwin-arm64
  freebsd-386
  freebsd-amd64
  freebsd-arm64
  linux-386
  linux-amd64
  linux-arm
  linux-arm64
  windows-386
  windows-amd64
  windows-arm64
)

IFS=$'\n' read -d '' -r -a supported_platforms < <(go tool dist list) || true

for p in "${platforms[@]}"; do
  goos="${p%-*}"
  goarch="${p#*-}"
  if [[ " ${supported_platforms[*]} " != *" ${goos}/${goarch} "* ]]; then
    echo "warning: skipping unsupported platform $p" >&2
    continue
  fi
  ext=""
  if [ "$goos" = "windows" ]; then
    ext=".exe"
  fi
  echo "building ${goos}/${goarch}..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" -o "dist/${p}${ext}"
done
