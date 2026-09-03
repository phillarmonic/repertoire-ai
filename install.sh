#!/usr/bin/env bash
set -euo pipefail

repository="phillarmonic/repertoire-ai"
binary_name="repertoire"
install_dir="${INSTALL_DIR:-${HOME}/.local/bin}"
requested_version="${1:-}"
temporary_dir=""


cleanup() {
    if [[ -n "$temporary_dir" && -d "$temporary_dir" ]]; then
        rm -rf "$temporary_dir"
    fi
}
trap cleanup EXIT

fail() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

case "$(uname -s)" in
    Linux*) target_os="linux" ;;
    Darwin*) target_os="darwin" ;;
    MINGW*|MSYS*|CYGWIN*) target_os="windows" ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
    x86_64|amd64) target_arch="amd64" ;;
    arm64|aarch64) target_arch="arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || fail "curl is required"

if [[ -z "$requested_version" ]]; then
    requested_version="$(
        curl -fsSL "https://api.github.com/repos/${repository}/releases/latest" |
            sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
            head -n 1
    )"
fi

if [[ ! "$requested_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
    fail "invalid release version: ${requested_version:-<empty>}"
fi

asset="${binary_name}-${target_os}-${target_arch}"
installed_name="$binary_name"
if [[ "$target_os" == "windows" ]]; then
    asset="${asset}.exe"
    installed_name="${installed_name}.exe"
fi

release_url="https://github.com/${repository}/releases/download/${requested_version}"
temporary_dir="$(mktemp -d)"

printf 'Installing Repertoire %s for %s/%s\n' "$requested_version" "$target_os" "$target_arch"
curl -fsSL "${release_url}/${asset}" -o "${temporary_dir}/${asset}"
curl -fsSL "${release_url}/checksums.txt" -o "${temporary_dir}/checksums.txt"

expected_checksum="$(
    awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' \
        "${temporary_dir}/checksums.txt"
)"
[[ -n "$expected_checksum" ]] || fail "checksum not found for ${asset}"

if command -v sha256sum >/dev/null 2>&1; then
    actual_checksum="$(sha256sum "${temporary_dir}/${asset}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
    actual_checksum="$(shasum -a 256 "${temporary_dir}/${asset}" | awk '{print $1}')"
else
    fail "sha256sum or shasum is required to verify the download"
fi

[[ "$actual_checksum" == "$expected_checksum" ]] || fail "checksum verification failed"

mkdir -p "$install_dir"
destination="${install_dir}/${installed_name}"
install -m 0755 "${temporary_dir}/${asset}" "$destination"

version_output="$("$destination" --version)"
# Release binaries stamp either the rich "vX.Y.Z (commit, date)" string (Unix,
# built by scripts/build-release.sh) or the plain "vX.Y.Z" string (Windows,
# built by the drun installer task), so accept both forms.
[[ "$version_output" == "repertoire version ${requested_version}" || \
   "$version_output" == "repertoire version ${requested_version} ("* ]] ||
    fail "installed binary reported an unexpected version: ${version_output}"

printf 'Installed %s\n' "$destination"
if [[ ":${PATH}:" != *":${install_dir}:"* ]]; then
    printf 'Add %s to PATH to run repertoire from any shell.\n' "$install_dir"
fi
