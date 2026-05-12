param (
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

$Repo       = "marcodellemarche/cubbit-pages"
$Binary     = "cubbit-pages"
$InstallDir = "$env:LOCALAPPDATA\cubbit-pages"

# Detect architecture
switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64"  { $target = "windows-amd64" }
    "ARM64"  { $target = "windows-arm64" }
    default  { Write-Error "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)"; exit 1 }
}

$filename = "$Binary-$target.exe"

# Resolve version
if ($Version -eq "latest") {
    $release = Invoke-RestMethod `
        -Uri "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "cubbit-pages-install" }
    $Version = $release.tag_name
}

$baseUrl     = "https://github.com/$Repo/releases/download/$Version"
$downloadUrl = "$baseUrl/$filename"
$checksumUrl = "$baseUrl/$filename.sha256"

Write-Host "Downloading $Binary $Version for windows/$($target.Split('-')[1])..."

$tmpDir = Join-Path $env:TEMP "cubbit-pages-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir | Out-Null

try {
    $binPath = Join-Path $tmpDir $filename
    $csPath  = Join-Path $tmpDir "$filename.sha256"

    Invoke-WebRequest -Uri $downloadUrl -OutFile $binPath -UseBasicParsing
    Invoke-WebRequest -Uri $checksumUrl -OutFile $csPath  -UseBasicParsing

    # Verify SHA256 checksum
    Write-Host "Verifying checksum..."
    $expectedHash = (Get-Content $csPath -Raw).Trim().Split()[0].ToLower()
    $actualHash   = (Get-FileHash $binPath -Algorithm SHA256).Hash.ToLower()
    if ($expectedHash -ne $actualHash) {
        throw "Checksum mismatch! Expected: $expectedHash  Got: $actualHash"
    }
    Write-Host "Checksum OK"

    # Install
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destPath = Join-Path $InstallDir "$Binary.exe"
    Copy-Item $binPath $destPath -Force

    # Add to PATH (User scope, persists across sessions)
    $currentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    if ($currentPath -notlike "*$InstallDir*") {
        [System.Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to PATH"
    }
    # Update PATH for current session too
    if ($env:PATH -notlike "*$InstallDir*") {
        $env:PATH = "$env:PATH;$InstallDir"
    }

    Write-Host ""
    Write-Host "cubbit-pages $Version installed to $destPath"
    Write-Host ""
    Write-Host "Restart your terminal for PATH changes to take effect, or run:"
    Write-Host "  `$env:PATH += `";$InstallDir`""
} finally {
    Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
