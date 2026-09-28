# Install fotoly and pixelfox from GitHub releases.
# Windows:
#   powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.ps1 | iex"
# Pin a version or directory before the command:
#   $env:FOTOLY_VERSION = '1.0.0'
#   $env:FOTOLY_INSTALL_DIR = "$HOME\.local\bin"
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'ManuelReschke/fotoly-cli'
$version = $env:FOTOLY_VERSION
$dest = $env:FOTOLY_INSTALL_DIR

function Fail([string]$Message) {
    Write-Error "install.ps1: $Message"
    exit 1
}

if (-not $dest) {
    if (-not $env:USERPROFILE) { Fail 'USERPROFILE is not set; set FOTOLY_INSTALL_DIR' }
    $dest = Join-Path $env:USERPROFILE '.local\bin'
}

$proc = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) { $proc = $env:PROCESSOR_ARCHITEW6432 }
switch ($proc) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { Fail "unsupported architecture: $proc" }
}

if (-not $version) {
    $request = [System.Net.WebRequest]::Create("https://github.com/$repo/releases/latest")
    $request.AllowAutoRedirect = $false
    $request.UserAgent = 'fotoly-cli-installer'
    $response = $request.GetResponse()
    $location = [string]$response.Headers['Location']
    $response.Close()
    if (-not $location) { Fail 'could not resolve the latest release' }
    $version = ($location.TrimEnd('/') -split '/')[-1]
}
$version = $version -replace '^v', ''
if ($version -notmatch '^[0-9A-Za-z][0-9A-Za-z._+-]*$') { Fail "invalid version: $version" }
$tag = "v$version"
$asset = "fotoly-cli_${version}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$tag"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("fotoly-cli-install-" + [guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "Installing fotoly and pixelfox $tag for windows/$arch"
    $sumsPath = Join-Path $tmp 'SHA256SUMS'
    $zipPath = Join-Path $tmp $asset
    $headers = @{ 'User-Agent' = 'fotoly-cli-installer' }
    Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile $sumsPath -UseBasicParsing -Headers $headers
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zipPath -UseBasicParsing -Headers $headers

    $expected = $null
    foreach ($line in Get-Content -Path $sumsPath) {
        $parts = $line -split '\s+'
        if ($parts.Length -ge 2 -and $parts[1] -eq $asset) {
            $expected = $parts[0]
            break
        }
    }
    if (-not $expected) { Fail "no checksum published for $asset" }
    $actual = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash
    if ($actual.ToLowerInvariant() -ne $expected.ToLowerInvariant()) { Fail "checksum mismatch for $asset" }

    $extract = Join-Path $tmp 'extract'
    Expand-Archive -Path $zipPath -DestinationPath $extract -Force
    $stem = "fotoly-cli_${version}_windows_${arch}"
    $fotoly = Join-Path $extract "$stem\fotoly.exe"
    $pixelfox = Join-Path $extract "$stem\pixelfox.exe"
    if (-not (Test-Path $fotoly) -or -not (Test-Path $pixelfox)) { Fail 'archive is missing fotoly.exe or pixelfox.exe' }

    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item -Force $fotoly (Join-Path $dest 'fotoly.exe')
    Copy-Item -Force $pixelfox (Join-Path $dest 'pixelfox.exe')
}
finally {
    Remove-Item -Recurse -Force $tmp
}

Write-Host "Installed $tag to $dest"
Write-Host "  $(Join-Path $dest 'fotoly.exe')"
Write-Host "  $(Join-Path $dest 'pixelfox.exe')"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $userPath) { $userPath = '' }
$parts = $userPath -split ';' | Where-Object { $_ -and ($_.TrimEnd('\') -ne $dest.TrimEnd('\')) }
if (($userPath -split ';' | ForEach-Object { $_.TrimEnd('\') }) -notcontains $dest.TrimEnd('\')) {
    $updated = (@($parts) + $dest) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    $env:Path = "$env:Path;$dest"
    Write-Host ""
    Write-Host "Added $dest to your user PATH. Open a new terminal so it takes effect."
}
Write-Host ""
Write-Host "Next: fotoly setup"
