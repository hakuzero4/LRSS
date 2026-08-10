# Wait until the Vite dev server responds on IPv4 loopback.
param(
    [int]$Port = 9245,
    [int]$TimeoutSec = 60
)

$url = "http://127.0.0.1:$Port/"
$deadline = (Get-Date).AddSeconds($TimeoutSec)
Write-Host "Waiting for frontend at $url (timeout ${TimeoutSec}s)..."

while ((Get-Date) -lt $deadline) {
    try {
        $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 2
        if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500) {
            Write-Host "frontend ready: $url"
            exit 0
        }
    } catch {
        # not ready yet
    }
    Start-Sleep -Milliseconds 500
}

Write-Error "timeout waiting for $url"
exit 1
