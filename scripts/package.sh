#!/bin/sh
set -eu

# Stable version tags only; keep metadata safe for Go's linker flag parser.
: "${VERSION:?set VERSION to vX.Y.Z}"
printf '%s\n' "$VERSION" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || {
  echo "VERSION must be a stable vX.Y.Z tag" >&2
  exit 1
}
BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse HEAD)}
BUILD_TIME=${BUILD_TIME:-$(git show -s --format=%cI "$BUILD_COMMIT")}
printf '%s\n' "$BUILD_COMMIT" | grep -Eq '^[0-9a-f]{40}$'
printf '%s\n' "$BUILD_TIME" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})$'
export VERSION BUILD_COMMIT BUILD_TIME
release_dir="dist/release/$VERSION"
mkdir -p "$release_dir"

for target in linux/amd64/linux/x64 linux/arm64/linux/arm64 darwin/amd64/macos/x64 darwin/arm64/macos/arm64; do
  goos=${target%%/*}; rest=${target#*/}
  goarch=${rest%%/*}; rest=${rest#*/}
  platform=${rest%%/*}; arch=${rest#*/}
  stage="dist/stage/$goos-$goarch"
  mkdir -p "$stage/bin"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
    -ldflags "-s -w -X main.buildVersion=${VERSION#v} -X main.buildCommit=$BUILD_COMMIT -X main.buildTime=$BUILD_TIME" \
    -o "$stage/bin/tell" ./cmd/tell
  # List the binary explicitly: no stale staging files enter the archive.
  tar -C "$stage" -czf "$release_dir/tell-$VERSION-$platform-$arch.tar.gz" bin/tell
done

(
  cd "$release_dir"
  shasum -a 256 "tell-$VERSION-linux-x64.tar.gz" "tell-$VERSION-linux-arm64.tar.gz" \
    "tell-$VERSION-macos-x64.tar.gz" "tell-$VERSION-macos-arm64.tar.gz" > checksums.txt
)
sh scripts/check-package.sh
