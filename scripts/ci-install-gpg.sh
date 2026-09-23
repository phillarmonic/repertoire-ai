#!/usr/bin/env bash
# Install GnuPG and put gpg on PATH for later GitHub Actions steps.
set -euo pipefail

case "${RUNNER_OS:-}" in
Linux)
	sudo DEBIAN_FRONTEND=noninteractive apt-get update
	sudo DEBIAN_FRONTEND=noninteractive apt-get install -y gnupg
	;;
macOS)
	brew install gnupg
	;;
Windows)
	choco install gnupg --yes --no-progress
	found=""
	while IFS= read -r candidate; do
		found="$candidate"
		break
	done < <(find "/c/Program Files" "/c/Program Files (x86)" -maxdepth 3 -type f -name gpg.exe 2>/dev/null || true)
	if [[ -z "$found" ]]; then
		echo "gpg.exe was not found after installing GnuPG" >&2
		exit 1
	fi
	cygpath -w "$(dirname "$found")" >>"$GITHUB_PATH"
	"$found" --version
	exit 0
	;;
*)
	echo "unsupported runner OS: ${RUNNER_OS:-unset}" >&2
	exit 1
	;;
esac

command -v gpg
gpg --version
