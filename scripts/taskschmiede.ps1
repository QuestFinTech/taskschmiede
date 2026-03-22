#
# Unified Taskschmiede management script (Windows).
# Builds, starts, stops, restarts, and reports status of the app server
# and MCP development proxy.
#
# Usage:
#   .\taskschmiede.ps1 <Command> [Target]
#
# Commands:
#   start    Build and start service(s) in the background
#   stop     Stop running service(s) gracefully
#   restart  Stop, rebuild, then start service(s)
#   status   Show running state of all services
#
# Targets (for start/stop/restart):
#   app      Taskschmiede server only (MCP + REST API)
#   proxy    MCP development proxy only
#   all      Both services (default when target omitted)
#
# Status always checks both services; target argument is ignored.
#
param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("start", "stop", "restart", "status")]
    [string]$Command,

    [Parameter(Position = 1)]
    [ValidateSet("app", "proxy", "all")]
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"

$ScriptDir  = $PSScriptRoot
$ProjectDir = Split-Path $ScriptDir -Parent
$RunDir     = Join-Path $ProjectDir "run"
$PidDir     = $ScriptDir

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

function Test-ProcessAlive {
    param([int]$ProcessId)
    try {
        $proc = Get-Process -Id $ProcessId -ErrorAction Stop
        return -not $proc.HasExited
    } catch {
        return $false
    }
}

function Get-ProcessUptime {
    param([int]$ProcessId)
    try {
        $proc = Get-Process -Id $ProcessId -ErrorAction Stop
        $span = (Get-Date) - $proc.StartTime
        if ($span.Days -gt 0) {
            return "{0}-{1:d2}:{2:d2}:{3:d2}" -f $span.Days, $span.Hours, $span.Minutes, $span.Seconds
        }
        return "{0:d2}:{1:d2}:{2:d2}" -f [int]$span.TotalHours, $span.Minutes, $span.Seconds
    } catch {
        return "unknown"
    }
}

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------

function Test-Prerequisites {
    if (-not (Test-Path (Join-Path $RunDir "config.yaml"))) {
        Write-Host "Error: $RunDir\config.yaml not found. Copy from config.yaml.example first." -ForegroundColor Red
        exit 1
    }

    $envFile = Join-Path $RunDir ".env"
    if (-not (Test-Path $envFile)) {
        Write-Host "Error: $envFile not found." -ForegroundColor Red
        exit 1
    }

    # Load environment variables from .env
    # Supports: KEY=value, KEY="value", KEY='literal', KEY=$OTHER_KEY
    Get-Content $envFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -match '^\s*#' -or $line -eq '') { return }
        if ($line -match '^([^=]+)=(.*)$') {
            $key = $Matches[1].Trim()
            $val = $Matches[2].Trim()
            $literal = $false
            if ($val -match "^'(.*)'$") {
                $val = $Matches[1]
                $literal = $true
            } elseif ($val -match '^"(.*)"$') {
                $val = $Matches[1]
            }
            if (-not $literal) {
                $val = [regex]::Replace($val, '\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)', {
                    param($m)
                    $ref = if ($m.Groups[1].Success) { $m.Groups[1].Value } else { $m.Groups[2].Value }
                    $resolved = [Environment]::GetEnvironmentVariable($ref, 'Process')
                    if ($null -ne $resolved) { $resolved } else { $m.Value }
                })
            }
            [Environment]::SetEnvironmentVariable($key, $val, 'Process')
        }
    }
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

function Invoke-BuildApp {
    Write-Host "Building Taskschmiede..."
    Push-Location $ProjectDir
    try {
        & make build
        if ($LASTEXITCODE -ne 0) { Write-Host "Error: Build failed." -ForegroundColor Red; exit 1 }
    } finally { Pop-Location }

    if (-not (Test-Path $RunDir)) { New-Item -ItemType Directory -Path $RunDir -Force | Out-Null }
    $src = Join-Path (Join-Path (Join-Path $ProjectDir "build") "windows-amd64") "taskschmiede.exe"
    Copy-Item -Path $src -Destination (Join-Path $RunDir "taskschmiede.exe") -Force
}

function Invoke-BuildProxy {
    Write-Host "Building MCP proxy..."
    Push-Location $ProjectDir
    try {
        & make build-proxy
        if ($LASTEXITCODE -ne 0) { Write-Host "Error: Build failed." -ForegroundColor Red; exit 1 }
    } finally { Pop-Location }

    if (-not (Test-Path $RunDir)) { New-Item -ItemType Directory -Path $RunDir -Force | Out-Null }
    $src = Join-Path (Join-Path (Join-Path $ProjectDir "build") "windows-amd64") "taskschmiede-proxy.exe"
    Copy-Item -Path $src -Destination (Join-Path $RunDir "taskschmiede-proxy.exe") -Force
}

# ---------------------------------------------------------------------------
# Start
# ---------------------------------------------------------------------------

function Start-App {
    $pidFile = Join-Path $PidDir "server.pid"
    if (Test-Path $pidFile) {
        $existingPid = [int](Get-Content $pidFile)
        if (Test-ProcessAlive $existingPid) {
            Write-Host "Error: Server already running (PID $existingPid). Stop first." -ForegroundColor Red
            exit 1
        }
        Remove-Item $pidFile
    }

    Write-Host "Starting Taskschmiede server..."
    $proc = Start-Process `
        -FilePath (Join-Path $RunDir "taskschmiede.exe") `
        -ArgumentList "serve", "--config-file", "config.yaml" `
        -WorkingDirectory $RunDir `
        -NoNewWindow -PassThru `
        -RedirectStandardOutput (Join-Path $RunDir "server.out") `
        -RedirectStandardError (Join-Path $RunDir "server.err")

    Set-Content -Path $pidFile -Value $proc.Id -NoNewline
    Write-Host "  Server PID: $($proc.Id) (MCP :9000, REST API :9000)"

    Start-Sleep -Seconds 1
    if (-not (Test-ProcessAlive $proc.Id)) {
        Write-Host "Error: Server failed to start. Check $RunDir\server.out and server.err" -ForegroundColor Red
        Remove-Item $pidFile -ErrorAction SilentlyContinue
        exit 1
    }
}

function Start-Proxy {
    $pidFile = Join-Path $PidDir "proxy.pid"
    if (Test-Path $pidFile) {
        $existingPid = [int](Get-Content $pidFile)
        if (Test-ProcessAlive $existingPid) {
            Write-Host "Error: Proxy already running (PID $existingPid). Stop first." -ForegroundColor Red
            exit 1
        }
        Remove-Item $pidFile
    }

    Write-Host "Starting MCP proxy..."
    $proc = Start-Process `
        -FilePath (Join-Path $RunDir "taskschmiede-proxy.exe") `
        -ArgumentList "--config-file", "config.yaml" `
        -WorkingDirectory $RunDir `
        -NoNewWindow -PassThru `
        -RedirectStandardOutput (Join-Path $RunDir "proxy.out") `
        -RedirectStandardError (Join-Path $RunDir "proxy.err")

    Set-Content -Path $pidFile -Value $proc.Id -NoNewline
    Write-Host "  Proxy PID:  $($proc.Id) (listen :9001, upstream localhost:9000)"

    Start-Sleep -Seconds 1
    if (-not (Test-ProcessAlive $proc.Id)) {
        Write-Host "Error: Proxy failed to start. Check $RunDir\proxy.out and proxy.err" -ForegroundColor Red
        Remove-Item $pidFile -ErrorAction SilentlyContinue
        exit 1
    }
}

# ---------------------------------------------------------------------------
# Stop
# ---------------------------------------------------------------------------

function Stop-Component {
    param(
        [string]$Name,
        [string]$PidFile
    )

    if (-not (Test-Path $PidFile)) {
        Write-Host "No $Name PID file found."
        return
    }

    $processId = [int](Get-Content $PidFile)

    if (-not (Test-ProcessAlive $processId)) {
        Write-Host "$Name not running (stale PID $processId)."
        Remove-Item $PidFile
        return
    }

    Write-Host "Stopping $Name (PID $processId)..."
    try {
        Stop-Process -Id $processId -ErrorAction Stop
    } catch {
        # Process may have already exited
    }

    for ($i = 0; $i -lt 10; $i++) {
        if (-not (Test-ProcessAlive $processId)) { break }
        Start-Sleep -Milliseconds 500
    }

    if (Test-ProcessAlive $processId) {
        Write-Host "  Force killing $Name..."
        try {
            Stop-Process -Id $processId -Force -ErrorAction Stop
        } catch {
            # Ignore -- process may have exited between check and kill
        }
    }

    Write-Host "  $Name stopped."
    Remove-Item $PidFile -ErrorAction SilentlyContinue
    $script:stopped = $true
}

function Stop-App {
    Stop-Component -Name "Taskschmiede server" -PidFile (Join-Path $PidDir "server.pid")
}

function Stop-Proxy {
    Stop-Component -Name "MCP proxy" -PidFile (Join-Path $PidDir "proxy.pid")
}

# ---------------------------------------------------------------------------
# Status
# ---------------------------------------------------------------------------

function Show-Status {
    Write-Host "Taskschmiede Status"
    Write-Host "-------------------------------------------"

    # App server
    $serverPidFile = Join-Path $PidDir "server.pid"
    if (Test-Path $serverPidFile) {
        $appPid = [int](Get-Content $serverPidFile)
        if (Test-ProcessAlive $appPid) {
            $uptime = Get-ProcessUptime $appPid
            Write-Host "App server:  RUNNING  PID $appPid  uptime $uptime"
            Write-Host "  MCP:       http://localhost:9000/mcp"
            Write-Host "  REST API:  http://localhost:9000/api/v1/"
            try {
                $health = Invoke-RestMethod -Uri "http://localhost:9000/mcp/health" -TimeoutSec 2 -ErrorAction Stop
                Write-Host "  Health:    $($health.status)"
            } catch {
                Write-Host "  Health:    unreachable"
            }
        } else {
            Write-Host "App server:  STOPPED  (stale PID file)"
            Remove-Item $serverPidFile
        }
    } else {
        Write-Host "App server:  STOPPED"
    }

    Write-Host ""

    # MCP proxy
    $proxyPidFile = Join-Path $PidDir "proxy.pid"
    if (Test-Path $proxyPidFile) {
        $proxyPid = [int](Get-Content $proxyPidFile)
        if (Test-ProcessAlive $proxyPid) {
            $uptime = Get-ProcessUptime $proxyPid
            Write-Host "MCP proxy:   RUNNING  PID $proxyPid  uptime $uptime"
            Write-Host "  Listen:    http://localhost:9001/mcp"
            try {
                $health = Invoke-RestMethod -Uri "http://localhost:9001/proxy/health" -TimeoutSec 2 -ErrorAction Stop
                Write-Host "  Upstream:  $($health.upstream_status)"
                Write-Host "  Clients:   $($health.clients)"
            } catch {
                Write-Host "  Upstream:  unreachable"
            }
        } else {
            Write-Host "MCP proxy:   STOPPED  (stale PID file)"
            Remove-Item $proxyPidFile
        }
    } else {
        Write-Host "MCP proxy:   STOPPED"
    }

    Write-Host "-------------------------------------------"
    Write-Host "Logs:"
    Write-Host "  Server:    $RunDir\server.out"
    Write-Host "  Proxy:     $RunDir\proxy.out"
    Write-Host "  Traffic:   $RunDir\taskschmiede-mcp-traffic.log"
}

# ---------------------------------------------------------------------------
# Summary (after start/restart)
# ---------------------------------------------------------------------------

function Write-Summary {
    Write-Host ""
    Write-Host "Taskschmiede is running:"
    if ($Target -eq "all" -or $Target -eq "app") {
        Write-Host "  MCP server:   http://localhost:9000/mcp"
        Write-Host "  REST API:     http://localhost:9000/api/v1/"
    }
    if ($Target -eq "all" -or $Target -eq "proxy") {
        Write-Host "  MCP proxy:    http://localhost:9001/mcp"
        Write-Host "  Proxy health: http://localhost:9001/proxy/health"
    }
    Write-Host ""
    Write-Host "Logs:"
    if ($Target -eq "all" -or $Target -eq "app") {
        Write-Host "  Server: $RunDir\server.out"
    }
    if ($Target -eq "all" -or $Target -eq "proxy") {
        Write-Host "  Proxy:  $RunDir\proxy.out"
        Write-Host "  Traffic: $RunDir\taskschmiede-mcp-traffic.log"
    }
    Write-Host ""
    Write-Host "Run .\taskschmiede.ps1 stop to stop services."
}

# ---------------------------------------------------------------------------
# Command dispatch
# ---------------------------------------------------------------------------

switch ($Command) {
    "start" {
        Test-Prerequisites
        switch ($Target) {
            "app"   { Invoke-BuildApp; Start-App }
            "proxy" { Invoke-BuildProxy; Start-Proxy }
            "all"   { Invoke-BuildApp; Invoke-BuildProxy; Start-App; Start-Proxy }
        }
        Write-Summary
    }
    "stop" {
        $script:stopped = $false
        switch ($Target) {
            "app"   { Stop-App }
            "proxy" { Stop-Proxy }
            "all"   { Stop-Proxy; Stop-App }
        }
        if (-not $script:stopped) {
            Write-Host "Nothing was running."
        }
    }
    "restart" {
        $script:stopped = $false
        switch ($Target) {
            "app"   { Stop-App }
            "proxy" { Stop-Proxy }
            "all"   { Stop-Proxy; Stop-App }
        }
        Test-Prerequisites
        switch ($Target) {
            "app"   { Invoke-BuildApp; Start-App }
            "proxy" { Invoke-BuildProxy; Start-Proxy }
            "all"   { Invoke-BuildApp; Invoke-BuildProxy; Start-App; Start-Proxy }
        }
        Write-Summary
    }
    "status" {
        Show-Status
    }
}
