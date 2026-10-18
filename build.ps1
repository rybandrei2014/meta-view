# Cross-compile release binaries for Windows, macOS and Linux.
# Requires only the Go toolchain - no C compiler, no other dependencies.
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
foreach ($t in $targets) {
    $out = "dist/meta-view-$($t.GOOS)-$($t.GOARCH)$($t.EXT)"
    $env:GOOS = $t.GOOS; $env:GOARCH = $t.GOARCH; $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags "-s -w" -o $out .
    if (-not $?) { throw "build failed for $($t.GOOS)/$($t.GOARCH)" }
    Write-Host "built $out"
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
