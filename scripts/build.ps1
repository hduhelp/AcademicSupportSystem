# PowerShell build script for Windows
# Builds frontend and backend for Linux deployment

$ErrorActionPreference = "Stop"

# Get project root directory
$ROOT_DIR = Split-Path -Parent $PSScriptRoot
Set-Location $ROOT_DIR

Write-Host "Cleaning up old build directory..." -ForegroundColor Cyan
if (Test-Path "build") {
    Remove-Item -Recurse -Force "build"
}
New-Item -ItemType Directory -Path "build/static" -Force | Out-Null

# Build frontend
Write-Host "Building frontend..." -ForegroundColor Cyan
Set-Location "frontend"

# Install dependencies if node_modules doesn't exist
if (-not (Test-Path "node_modules")) {
    Write-Host "Installing frontend dependencies..." -ForegroundColor Yellow
    npm install
}

npm run build
Write-Host "Frontend built successfully." -ForegroundColor Green

# Move frontend assets
Write-Host "Moving frontend assets..." -ForegroundColor Cyan
Set-Location $ROOT_DIR
Copy-Item -Recurse -Force "frontend/build/*" "build/static/"
Remove-Item -Recurse -Force "frontend/build"

# Build backend for Linux
Write-Host "Building backend for Linux..." -ForegroundColor Cyan
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o build/server main.go
Write-Host "Backend built successfully." -ForegroundColor Green

# Clear environment variables
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Build complete! All artifacts are in the 'build' directory." -ForegroundColor Green
Write-Host "   - Backend executable: build/server" -ForegroundColor White
Write-Host "   - Frontend assets: build/static/" -ForegroundColor White
