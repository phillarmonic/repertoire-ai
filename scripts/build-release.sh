#!/usr/bin/env bash
set -euo pipefail

version="${1:-dev}"
commit="${GITHUB_SHA:-$(git rev-parse HEAD 2>/dev/null || printf 'unknown')}"
short_commit="${commit:0:7}"
build_date="${BUILD_DATE:-$(date -u '+%Y-%m-%d %H:%M:%S UTC')}"
display_version="${version} (${short_commit}, ${build_date})"

if [[ "$version" != "dev" && ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
    printf 'error: version must be dev or a semantic version prefixed with v\n' >&2
    exit 1
fi

mkdir -p dist
find dist -maxdepth 1 -type f \( -name 'repertoire-*' -o -name 'checksums.txt' \) -delete

platforms=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Windows binaries (windows/amd64, windows/arm64) are produced by the drun
# "installer" task, which also stamps the version via -ldflags and bundles them
# into the per-user NSIS setup executable.

printf 'Building Repertoire %s\n' "$display_version"

for platform in "${platforms[@]}"; do
    IFS=/ read -r target_os target_arch <<< "$platform"
    asset="repertoire-${target_os}-${target_arch}"
    if [[ "$target_os" == "windows" ]]; then
        asset="${asset}.exe"
    fi

    printf '  %s\n' "$asset"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
        go build \
        -trimpath \
        -ldflags="-s -w -buildid= -X 'main.version=${display_version}'" \
        -o "dist/${asset}" \
        ./cmd/repertoire
done

if command -v sha256sum >/dev/null 2>&1; then
    (
        cd dist
        sha256sum repertoire-* > checksums.txt
    )
elif command -v shasum >/dev/null 2>&1; then
    (
        cd dist
        shasum -a 256 repertoire-* > checksums.txt
    )
else
    printf 'error: sha256sum or shasum is required\n' >&2
    exit 1
fi

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"
host_asset="dist/repertoire-${host_os}-${host_arch}"
if [[ "$host_os" == "windows" ]]; then
    host_asset="${host_asset}.exe"
fi

if [[ -x "$host_asset" ]]; then
    version_output="$("$host_asset" --version)"
    expected="repertoire version ${display_version}"
    if [[ "$version_output" != "$expected" ]]; then
        printf 'error: expected %q, got %q\n' "$expected" "$version_output" >&2
        exit 1
    fi
    "$host_asset" --help >/dev/null
fi

printf 'Built %s release assets and checksums.txt\n' "${#platforms[@]}"
