<#
.SYNOPSIS
    Install the Repertoire CLI for the current Windows user (no elevation).

.DESCRIPTION
    Downloads the prebuilt Repertoire binary that matches the machine's native
    architecture (amd64 or arm64), verifies its SHA-256 checksum against the
    published checksums.txt, installs it to %LOCALAPPDATA%\Programs\Repertoire
    (the same location the NSIS setup uses), and adds that directory to the
    current user's PATH. No administrator privileges are required.

.PARAMETER Version
    Release tag to install, for example v1.2.3. Defaults to the latest release
    (or the REPERTOIRE_VERSION environment variable when set).

.PARAMETER InstallDir
    Target directory. Defaults to %LOCALAPPDATA%\Programs\Repertoire.

.EXAMPLE
    irm https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -Version v1.2.3
#>
[CmdletBinding()]
param(
    [string]$Version = $env:REPERTOIRE_VERSION,
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\Repertoire')
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repository = 'phillarmonic/repertoire-ai'
$binaryName = 'repertoire'

function Fail($message) {
    Write-Error $message
    exit 1
}

# Enable TLS 1.2 for Windows PowerShell 5.1, which does not negotiate it by default.
try {
    [Net.ServicePointManager]::SecurityProtocol = `
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
}

# Detect the machine's native architecture. PROCESSOR_ARCHITEW6432 is set when a
# 32-bit process runs on a 64-bit OS and reports the true architecture.
$nativeArch = $env:PROCESSOR_ARCHITEW6432
if (-not $nativeArch) { $nativeArch = $env:PROCESSOR_ARCHITECTURE }
switch ($nativeArch) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { Fail "unsupported architecture: $nativeArch" }
}

if (-not $Version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repository/releases/latest" `
        -Headers @{ 'User-Agent' = 'repertoire-installer' }
    $Version = $release.tag_name
}
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$') {
    Fail "invalid release version: $(if ($Version) { $Version } else { '<empty>' })"
}

$asset = "$binaryName-windows-$arch.exe"
$releaseUrl = "https://github.com/$repository/releases/download/$Version"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("repertoire-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    Write-Host "Installing Repertoire $Version for windows/$arch"

    $assetPath = Join-Path $tempDir $asset
    $sumsPath = Join-Path $tempDir 'checksums.txt'
    Invoke-WebRequest -Uri "$releaseUrl/$asset" -OutFile $assetPath -UseBasicParsing
    Invoke-WebRequest -Uri "$releaseUrl/checksums.txt" -OutFile $sumsPath -UseBasicParsing

    $checksumLine = Get-Content $sumsPath | Where-Object { $_ -match "\s\*?$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $checksumLine) { Fail "checksum not found for $asset" }
    $expectedHash = ($checksumLine -split '\s+')[0]
    $actualHash = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash
    if ($actualHash -ne $expectedHash) { Fail 'checksum verification failed' }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destination = Join-Path $InstallDir "$binaryName.exe"
    Copy-Item -Path $assetPath -Destination $destination -Force

    $versionOutput = (& $destination --version) -join "`n"
    if ($versionOutput -notmatch "repertoire version $([regex]::Escape($Version))(\s|$)") {
        Fail "installed binary reported an unexpected version: $versionOutput"
    }

    Write-Host "Installed $destination"

    # Add the install directory to the current user's PATH as its own segment,
    # de-duplicating so reruns never add it twice.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $segments = @()
    if ($userPath) { $segments = @($userPath -split ';' | Where-Object { $_ -ne '' }) }
    if ($segments -notcontains $InstallDir) {
        $newPath = (@($segments + $InstallDir) -join ';')
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $InstallDir to your user PATH. Open a new terminal to run 'repertoire'."
    } else {
        Write-Host "$InstallDir is already on your user PATH."
    }
}
finally {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
