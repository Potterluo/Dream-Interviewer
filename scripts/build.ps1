# build.ps1 — the Windows/cross-platform equivalent of `make build`.
#
# Why this exists: the whole frontend→embed→binary pipeline lived only in the
# Makefile, and `make` does not exist on Windows. Reimplementing the three
# lines by hand has two silent failure modes:
#
#   1. Forgetting to delete internal/server/dist first. `Copy-Item -Recurse`
#      into an existing directory MERGES, so stale chunks from a previous
#      build stay in the go:embed tree and ship inside the binary. The
#      symptom is "I rebuilt but the page is still old".
#   2. Forgetting MARKDOWN_FULL, which silently produces the lite build.
#
# It also restores internal/server/dist/.gitkeep, which the naive
# `Remove-Item -Recurse` deletes. Without it nothing about dist/ is tracked
# (the rest is gitignored), so a fresh clone has NO dist directory and
# `go build` fails outright on //go:embed all:dist.

[CmdletBinding()]
param(
    # Skip the Next.js build and reuse the existing web/out (faster iteration
    # on Go-only changes, but you must have built the web at least once).
    [switch]$SkipWeb,
    # Build the Wails desktop shell as well (bin/app-desktop.exe).
    [switch]$Desktop,
    # Build the lite-markdown variant (no Shiki, ~11 MB smaller) instead.
    [switch]$LiteMarkdown,
    [string]$Version = "dev",
    [string]$Commit = "local",
    [string]$Date = "dev",
    [string]$Output = "bin/app.exe"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# --- frontend ---------------------------------------------------------------
if (-not $SkipWeb) {
    Step "building the Next.js static export"
    Push-Location web
    try {
        if ($LiteMarkdown) { $env:MARKDOWN_FULL = "0" } else { $env:MARKDOWN_FULL = "1" }
        pnpm build
        if ($LASTEXITCODE -ne 0) { throw "pnpm build failed ($LASTEXITCODE)" }
    } finally {
        Pop-Location
    }
}

if (-not (Test-Path "web\out\index.html")) {
    throw "web/out/index.html is missing — run without -SkipWeb first."
}

# --- embed ------------------------------------------------------------------
Step "syncing web/out into internal/server/dist"
if (Test-Path "internal\server\dist") {
    # REPLACE, never merge. See the note at the top of this file.
    Remove-Item -Recurse -Force "internal\server\dist"
}
Copy-Item -Recurse "web\out" "internal\server\dist"
# The .gitkeep is the only tracked thing in dist/ (everything else is
# gitignored), and it is what keeps a fresh clone buildable.
New-Item -ItemType File -Force -Path "internal\server\dist\.gitkeep" | Out-Null

$distFiles = (Get-ChildItem "internal\server\dist" -Recurse -File).Count
if ($distFiles -lt 2) { throw "internal/server/dist looks empty ($distFiles files)" }
Step "dist now holds $distFiles files"

# --- binary -----------------------------------------------------------------
$ldflags = @(
    "-s", "-w",
    "-X", "main.version=$Version",
    "-X", "main.commit=$Commit",
    "-X", "main.date=$Date",
    "-X", "github.com/Potterluo/dream-interviewer/internal/buildinfo.Version=$Version",
    "-X", "github.com/Potterluo/dream-interviewer/internal/buildinfo.Commit=$Commit",
    "-X", "github.com/Potterluo/dream-interviewer/internal/buildinfo.Date=$Date"
)

Step "building $Output"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Output) | Out-Null
$env:CGO_ENABLED = "0"
# -trimpath is not cosmetic. Without it the binary embeds the absolute paths of
# the source files AND of every module-cache entry, e.g.
# C:\Users\<you>\go\pkg\mod\github.com\lib\pq@... — which publishes your
# username inside a downloadable exe. It also makes the build reproducible.
go build -trimpath -ldflags ($ldflags -join " ") -o $Output ./cmd/server
if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
$size = [math]::Round((Get-Item $Output).Length / 1MB, 1)
Step "$Output  $size MB  (version $Version)"

if ($Desktop) {
    Step "building bin/app-desktop.exe"
    $desktopLd = ($ldflags -join " ") + " -H windowsgui"
    go build -trimpath -tags "desktop,production" -ldflags $desktopLd -o "bin/app-desktop.exe" ./cmd/desktop
    if ($LASTEXITCODE -ne 0) { throw "desktop build failed ($LASTEXITCODE)" }
    Step "bin/app-desktop.exe  $([math]::Round((Get-Item 'bin/app-desktop.exe').Length / 1MB, 1)) MB"
}

Write-Host ""
Write-Host "done." -ForegroundColor Green
