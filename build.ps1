# Cross-compile release binaries for Windows, macOS and Linux, then package
# each into a zip with README and generate SHA256SUMS.
# Requires only the Go toolchain - no C compiler, no other dependencies.
param([string]$Version = "dev")
$ErrorActionPreference = "Stop"
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; EXT = ".exe" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; EXT = "" },
    @{ GOOS = "linux";   GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "linux";   GOARCH = "arm64"; EXT = "" }
)
New-Item -ItemType Directory -Force -Path dist | Out-Null
$ldflags = "-s -w -X main.version=$Version"
foreach ($t in $targets) {
    $out = "dist/meta-view-$($t.GOOS)-$($t.GOARCH)$($t.EXT)"
    $env:GOOS = $t.GOOS; $env:GOARCH = $t.GOARCH; $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags $ldflags -o $out .
    if (-not $?) { throw "build failed for $($t.GOOS)/$($t.GOARCH)" }
    Write-Host "built $out"
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED

# package: one zip per platform, binary named meta-view(.exe) at zip root
foreach ($t in $targets) {
    $bin = "dist/meta-view-$($t.GOOS)-$($t.GOARCH)$($t.EXT)"
    $zip = "dist/meta-view-$Version-$($t.GOOS)-$($t.GOARCH).zip"
    $stage = "dist/stage"
    Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    Copy-Item $bin (Join-Path $stage ("meta-view" + $t.EXT))
    Copy-Item README.md $stage
    Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -Force
    Remove-Item -Recurse -Force $stage
    Write-Host "packaged $zip"
}

# checksums (sha256sum-compatible format)
$lines = Get-ChildItem dist/*.zip | ForEach-Object {
    $h = (Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLower()
    "$h  $($_.Name)"
}
Set-Content -Path dist/SHA256SUMS.txt -Value $lines -Encoding ASCII
Write-Host "wrote dist/SHA256SUMS.txt"
