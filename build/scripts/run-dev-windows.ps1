param(
    [Parameter(Mandatory = $true)][string]$BinDir,
    [Parameter(Mandatory = $true)][string]$AppName
)

$port = $env:WAILS_VITE_PORT
if (-not $port) { $port = "9245" }

# Always prefer IPv4 — matches Vite host 127.0.0.1
$env:FRONTEND_DEVSERVER_URL = "http://127.0.0.1:$port"
Write-Host "FRONTEND_DEVSERVER_URL=$($env:FRONTEND_DEVSERVER_URL)"

$exe = Join-Path $BinDir "$AppName.exe"
if (-not (Test-Path $exe)) {
    Write-Error "binary not found: $exe"
    exit 1
}

& $exe
exit $LASTEXITCODE
