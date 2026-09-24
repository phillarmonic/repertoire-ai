#!/usr/bin/env bash
# Install GnuPG and put gpg on PATH for later GitHub Actions steps.
set -euo pipefail

# gpg_is_msys reports whether this gpg is the MSYS build. That build treats
# a Windows path such as C:\Users\... as a relative path.
gpg_is_msys() {
	ldd "$1" 2>/dev/null | grep -q 'msys-'
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
	# Scoop installs the native Windows GnuPG. Git Bash otherwise selects the
	# MSYS gpg from Git's usr\bin, which treats C:\... as a relative path.
	powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command '
		$ErrorActionPreference = "Stop"
		Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
		if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
			Invoke-Expression "& {$(Invoke-RestMethod -Uri https://get.scoop.sh)} -RunAsAdmin"
		}
		$env:PATH = "$env:USERPROFILE\scoop\shims;" + $env:PATH
		scoop config aria2-enabled false
		scoop install gnupg
		$prefix = (scoop prefix gnupg).Trim()
		$gpg = Get-ChildItem -Path $prefix -Recurse -Filter gpg.exe |
			Where-Object { $_.FullName -notmatch "\\shims\\" } |
			Select-Object -First 1
		if (-not $gpg) {
			throw "scoop gnupg did not provide gpg.exe"
		}
		Set-Content -Path (Join-Path $env:RUNNER_TEMP "scoop-gpg-bin.txt") -Encoding ascii -Value $gpg.DirectoryName
		& $gpg.FullName --version
	'

	bin="$(cygpath -u "$(tr -d '\r' <"$(cygpath -u "$RUNNER_TEMP")/scoop-gpg-bin.txt")")"
	if [[ ! -f "$bin/gpg.exe" ]]; then
		echo "scoop gpg.exe was not found in $bin" >&2
		exit 1
	fi
	if gpg_is_msys "$bin/gpg.exe"; then
		echo "scoop installed an MSYS gpg: $bin/gpg.exe" >&2
		exit 1
	fi

	# Git Bash prepends usr\bin and mingw64\bin after GITHUB_PATH is applied.
	# Move those MSYS binaries aside so later bash steps resolve gpg to Scoop.
	for candidate in /usr/bin/gpg.exe /mingw64/bin/gpg.exe; do
		if [[ -f "$candidate" ]] && gpg_is_msys "$candidate"; then
			mv "$candidate" "${candidate}.disabled-by-ci"
		fi
	done

	# Scoop's gpg shim forces GNUPGHOME to the package's home directory, which
	# hides the temporary keyring these tests create. Publish the real binary
	# only, and point Git at that same file.
	gpg_exe="$(cygpath -w "$bin/gpg.exe")"
	git config --system gpg.program "$gpg_exe"
	cygpath -w "$bin" >>"$GITHUB_PATH"
	export PATH="$bin:$PATH"
	;;
*)
	echo "unsupported runner OS: ${RUNNER_OS:-unset}" >&2
	exit 1
	;;
esac

command -v gpg
gpg --version
