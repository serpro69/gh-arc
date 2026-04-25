#!/usr/bin/env bash
set -e

TAG="${GITHUB_REF#refs/tags/}"
COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.Version=$TAG"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.GitCommit=$COMMIT"
LDFLAGS="$LDFLAGS -X github.com/serpro69/gh-arc/internal/version.BuildDate=$DATE"

go build -trimpath -ldflags "$LDFLAGS" -o "$1"
