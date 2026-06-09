param(
  [int]$FrontendPort = 3000,
  [int]$MockPort = 18080
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$Frontend = Join-Path $Root "frontend"
$Logs = Join-Path $env:TEMP "sub2api-laffey-preview"

New-Item -ItemType Directory -Force $Logs | Out-Null

function Test-PortListening {
  param([int]$Port)
  $Pattern = "^\s*TCP\s+\S+:$Port\s+\S+\s+LISTENING\s+\d+\s*$"
  return $null -ne (netstat -ano -p tcp | Select-String -Pattern $Pattern)
}

if (-not (Test-PortListening -Port $MockPort)) {
  $MockLog = Join-Path $Logs "mock.out.log"
  $MockCommand = @"
`$env:PREVIEW_API_HOST = '127.0.0.1'
`$env:PREVIEW_API_PORT = '$MockPort'
& 'node.exe' 'tools/frontend-preview-mock.mjs' *>&1 | Tee-Object -FilePath '$MockLog'
"@

  Start-Process `
    -FilePath "powershell.exe" `
    -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $MockCommand) `
    -WorkingDirectory $Root `
    -WindowStyle Hidden
}

if (Test-PortListening -Port $FrontendPort) {
  Write-Host "Frontend preview already listening: http://localhost:$FrontendPort/home"
  Write-Host "Mock API: http://127.0.0.1:$MockPort/api/v1/settings/public"
  Write-Host "Logs: $Logs"
  exit 0
}

$ViteLog = Join-Path $Logs "vite.out.log"
$ViteBin = Join-Path $Frontend "node_modules\.bin\vite.cmd"
$Command = @"
`$env:VITE_FRONTEND_PREVIEW = '1'
`$env:VITE_DEV_PROXY_TARGET = 'http://127.0.0.1:$MockPort'
`$env:VITE_DEV_PORT = '$FrontendPort'
& '$ViteBin' --host 0.0.0.0 --port $FrontendPort *>&1 | Tee-Object -FilePath '$ViteLog'
"@

Start-Process `
  -FilePath "powershell.exe" `
  -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $Command) `
  -WorkingDirectory $Frontend `
  -WindowStyle Hidden

Write-Host "Frontend preview: http://localhost:$FrontendPort/home"
Write-Host "Mock API: http://127.0.0.1:$MockPort/api/v1/settings/public"
Write-Host "Logs: $Logs"
