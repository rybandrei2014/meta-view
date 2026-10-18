#!/usr/bin/env sh
# Cross-compile release binaries for Windows, macOS and Linux.
# Requires only the Go toolchain - no C compiler, no other dependencies.
set -e
mkdir -p dist
for target in \
    windows/amd64:.exe windows/arm64:.exe \
    darwin/amd64: darwin/arm64: \
    linux/amd64: linux/arm64:
do
    osarch=${target%%:*}; ext=${target##*:}
    os=${osarch%%/*}; arch=${osarch##*/}
    out="dist/meta-view-$os-$arch$ext"
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$out" .
    echo "built $out"
done
