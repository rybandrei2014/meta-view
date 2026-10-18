#!/usr/bin/env sh
# Cross-compile release binaries for Windows, macOS and Linux, then package
# each into a zip with README and generate SHA256SUMS.
# Requires only the Go toolchain - no C compiler, no other dependencies.
set -e
VERSION=${VERSION:-dev}
mkdir -p dist
for target in \
    windows/amd64:.exe windows/arm64:.exe \
    darwin/amd64: darwin/arm64: \
    linux/amd64: linux/arm64:
do
    osarch=${target%%:*}; ext=${target##*:}
    os=${osarch%%/*}; arch=${osarch##*/}
    out="dist/meta-view-$os-$arch$ext"
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=$VERSION" -o "$out" .
    echo "built $out"
done

# package: one zip per platform, binary named meta-view(.exe) at zip root
for target in \
    windows/amd64:.exe windows/arm64:.exe \
    darwin/amd64: darwin/arm64: \
    linux/amd64: linux/arm64:
do
    osarch=${target%%:*}; ext=${target##*:}
    os=${osarch%%/*}; arch=${osarch##*/}
    bin="dist/meta-view-$os-$arch$ext"
    zip="dist/meta-view-$VERSION-$os-$arch.zip"
    rm -rf dist/stage && mkdir dist/stage
    cp "$bin" "dist/stage/meta-view$ext"
    cp README.md dist/stage
    (cd dist/stage && zip -q "../$(basename "$zip")" *)
    rm -rf dist/stage
    echo "packaged $zip"
done

# checksums (sha256sum-compatible format)
if command -v sha256sum >/dev/null 2>&1; then
    (cd dist && sha256sum ./*.zip | sed 's| \./|  |' > SHA256SUMS.txt)
else
    (cd dist && shasum -a 256 ./*.zip | sed 's| \./|  |' > SHA256SUMS.txt)
fi
echo "wrote dist/SHA256SUMS.txt"
