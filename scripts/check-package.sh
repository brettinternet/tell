#!/bin/sh
set -eu
: "${VERSION:?}" "${BUILD_COMMIT:?}" "${BUILD_TIME:?}"
release_dir="$(pwd)/dist/release/$VERSION"
(cd "$release_dir" && shasum -a 256 -c checksums.txt)
for platform in linux macos; do
  for arch in x64 arm64; do
    archive="$release_dir/tell-$VERSION-$platform-$arch.tar.gz"
    test "$(tar -tzf "$archive")" = bin/tell
  done
done
case "$(go env GOHOSTOS)" in
  darwin) platform=macos ;;
  linux) platform=linux ;;
  *) echo "Host smoke check requires Linux or macOS" >&2; exit 1 ;;
esac
case "$(go env GOHOSTARCH)" in
  amd64) arch=x64 ;;
  arm64) arch=arm64 ;;
  *) echo "Host smoke check requires x64 or arm64" >&2; exit 1 ;;
esac
# Extract into ignored build output; never consult personal Tell configuration.
mkdir -p dist/package-check
tar -xzf "$release_dir/tell-$VERSION-$platform-$arch.tar.gz" -C dist/package-check
actual="$(dist/package-check/bin/tell --version)"
short_commit="$(printf '%.7s' "$BUILD_COMMIT")"
expected="tell ${VERSION#v} ($short_commit, built $BUILD_TIME)"
test "$actual" = "$expected" || {
  printf 'Expected: %s\nActual: %s\n' "$expected" "$actual" >&2
  exit 1
}
test "$(dist/package-check/bin/tell version)" = "$expected"
printf '%s\n' "$actual"
