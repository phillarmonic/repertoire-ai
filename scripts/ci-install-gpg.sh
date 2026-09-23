#!/usr/bin/env bash
# Install GnuPG and put gpg on PATH for later GitHub Actions steps.
set -euo pipefail

# gpg_is_msys reports whether this gpg is the MSYS build. That build treats
# a Windows path such as C:\Users\... as a relative path. The MinGW gpg shipped
# in Git\mingw64\bin accepts the paths Go passes.
gpg_is_msys() {
	ldd "$1" 2>/dev/null | grep -q 'msys-'
}

# stage_windows_gpg publishes gpg without putting the rest of its bin
# directory on PATH. Git's usr/bin also contains find, sort, and other Unix
# tools that would shadow the Windows copies. gpg starts gpg-agent from the
# same directory, so that program and the private DLLs are copied too.
stage_windows_gpg() {
	local found="$1"
	local src dest tool dll
	src="$(cd "$(dirname "$found")" && pwd)"
	if [[ ! -f "$src/gpg.exe" ]]; then
		echo "gpg.exe was not found next to $found" >&2
		exit 1
	fi
	dest="$(cygpath -u "${RUNNER_TEMP:-/tmp}")/gpg-bin"
	mkdir -p "$dest"
	for tool in gpg.exe gpg-agent.exe gpgconf.exe gpg-connect-agent.exe; do
		if [[ -f "$src/$tool" ]]; then
			cp -f "$src/$tool" "$dest/$tool"
		fi
	done
	if command -v ldd >/dev/null 2>&1; then
		for tool in gpg.exe gpg-agent.exe gpgconf.exe gpg-connect-agent.exe; do
			[[ -f "$src/$tool" ]] || continue
			# Inspect the original binary. ldd on the copy resolves private
			# DLLs back into the staging directory and then cp refuses to
			# overwrite a file with itself.
			while IFS= read -r dll; do
				[[ -n "$dll" && -f "$dll" ]] || continue
				case "$dll" in
				"$dest"/* | /c/Windows/* | /c/WINDOWS/*) continue ;;
				esac
				cp -f "$dll" "$dest/"
			done < <(ldd "$src/$tool" | sed -n 's/.*=>[[:space:]]*//p' | awk '{print $1}')
		done
	else
		cp -f "$src/"*.dll "$dest/"
	fi
	echo "using existing GnuPG: $src/gpg.exe"
	cygpath -w "$dest" >>"$GITHUB_PATH"
	"$dest/gpg.exe" --version
}

case "${RUNNER_OS:-}" in
Linux)
	sudo DEBIAN_FRONTEND=noninteractive apt-get update
	sudo DEBIAN_FRONTEND=noninteractive apt-get install -y gnupg
	;;
macOS)
	brew install gnupg
	;;
Windows)
	# Prefer a native Windows gpg. Git Bash's `gpg` is the MSYS build from
	# usr\bin, and Chocolatey's gnupg package installs the full Gpg4win suite.
	found=""
	for candidate in \
		"/c/Program Files/Git/mingw64/bin/gpg.exe" \
		"/c/Program Files/GnuPG/bin/gpg.exe" \
		"/c/Program Files (x86)/GnuPG/bin/gpg.exe"; do
		if [[ -f "$candidate" ]] && ! gpg_is_msys "$candidate"; then
			found="$candidate"
			break
		fi
	done
	if [[ -z "$found" ]] && resolved="$(command -v gpg 2>/dev/null)"; then
		if [[ -f "${resolved}.exe" ]]; then
			resolved="${resolved}.exe"
		fi
		if [[ -f "$resolved" ]] && ! gpg_is_msys "$resolved"; then
			found="$resolved"
		fi
	fi
	if [[ -n "$found" ]]; then
		stage_windows_gpg "$found"
		exit 0
	fi

	echo "GnuPG was not preinstalled; installing with Chocolatey" >&2
	choco install gnupg --yes --no-progress
	while IFS= read -r candidate; do
		if ! gpg_is_msys "$candidate"; then
			found="$candidate"
			break
		fi
	done < <(find "/c/Program Files" "/c/Program Files (x86)" -maxdepth 4 -type f -name gpg.exe 2>/dev/null || true)
	if [[ -z "$found" ]]; then
		echo "gpg.exe was not found after installing GnuPG" >&2
		exit 1
	fi
	stage_windows_gpg "$found"
	exit 0
	;;
*)
	echo "unsupported runner OS: ${RUNNER_OS:-unset}" >&2
	exit 1
	;;
esac

command -v gpg
gpg --version
