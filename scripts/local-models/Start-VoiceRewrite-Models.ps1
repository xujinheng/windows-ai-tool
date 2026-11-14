#Requires -Version 5.1
<#!
.SYNOPSIS
  Launch Qwen3-ASR and Qwen3.5 using independent llama-server processes.
.DESCRIPTION
  Works with VoiceRewrite v19's original API endpoints: 8080 / 1234.
  Supports Start / Stop / Status / Test, dry-run, LM Studio GGUF discovery,
  and only stops processes that this script recorded and can identify.
.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File .\Start-VoiceRewrite-Models.ps1
.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File .\Start-VoiceRewrite-Models.ps1 -Action Stop
#>
[CmdletBinding()]
param(
    [ValidateSet('Start', 'Stop', 'Status', 'Test')]
    [string]$Action = 'Start',
    [ValidateSet('Both', 'ASR', 'LLM')]
    [string]$Service = 'Both',
    [string]$ConfigPath = (Join-Path $PSScriptRoot 'voice-servers.json'),
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ScriptRoot = $PSScriptRoot
$RuntimeDir = Join-Path $ScriptRoot 'runtime'
$StateFile = Join-Path $RuntimeDir 'server-state.json'
$SelectedRoles = if ($Service -eq 'Both') { @('ASR', 'LLM') } else { @($Service) }

function Expand-LocalPath([string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) { return '' }
    $result = [Environment]::ExpandEnvironmentVariables($Value.Trim())
    if ($result -eq '~') { $result = $env:USERPROFILE }
    elseif ($result.StartsWith('~\') -or $result.StartsWith('~/')) {
        $result = Join-Path $env:USERPROFILE $result.Substring(2)
    }
    if (-not [IO.Path]::IsPathRooted($result)) {
        $result = Join-Path $ScriptRoot $result
    }
    return [IO.Path]::GetFullPath($result)
}
function Assert-File([string]$Path, [string]$Label) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not [IO.File]::Exists($Path)) {
        throw "$Label not found: $Path"
    }
    return $Path
}
function Read-State {
    $table = @{}
    if (Test-Path -LiteralPath $StateFile -PathType Leaf) {
        try {
            $content = Get-Content -LiteralPath $StateFile -Encoding UTF8 -Raw | ConvertFrom-Json
            foreach ($property in $content.PSObject.Properties) {
                $table[$property.Name] = $property.Value
            }
        } catch { throw "Cannot read runtime state file: $StateFile. $($_.Exception.Message)" }
    }
    return ,$table
}
function Save-State([hashtable]$State) {
    if (-not (Test-Path -LiteralPath $RuntimeDir)) {
        $null = New-Item -ItemType Directory -Path $RuntimeDir -Force
    }
    $json = ConvertTo-Json -InputObject $State -Depth 6
    [IO.File]::WriteAllText($StateFile, $json, (New-Object Text.UTF8Encoding($false)))
}
function Get-RecordedProcess([string]$Role, [hashtable]$State) {
    if (-not $State.ContainsKey($Role)) { return $null }
    $record = $State[$Role]
    if ($null -eq $record -or $null -eq $record.pid) { return $null }
    $processId = [int]$record.pid
    try {
        $entry = Get-CimInstance Win32_Process -Filter "ProcessId=$processId" -ErrorAction Stop
        if ($null -eq $entry) { return $null }
        # Guard against PID reuse and against touching somebody else's server.
        $recordedServer = [string]$record.server
        $recordedModel = [string]$record.model
        if ([string]::IsNullOrWhiteSpace($entry.ExecutablePath) -or
            -not [string]::Equals($entry.ExecutablePath, $recordedServer, [StringComparison]::OrdinalIgnoreCase) -or
            [string]::IsNullOrWhiteSpace($entry.CommandLine) -or
            $entry.CommandLine.IndexOf($recordedModel, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
            return $null
        }
        return $entry
    } catch {
        # Conservative: if identity cannot be verified, never kill the PID.
        Write-Warning "Could not verify process identity for $Role / PID ${processId}: $($_.Exception.Message)"
        return $null
    }
}
function Test-LocalPort([int]$Port) {
    $client = New-Object Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect('127.0.0.1', $Port, $null, $null)
        $ready = $async.AsyncWaitHandle.WaitOne(300)
        if (-not $ready) { return $false }
        try { $client.EndConnect($async); return $true }
        catch { return $false }
    } catch { return $false }
    finally { $client.Close() }
}
function Quote-NativeArg([string]$Value) {
    if ($Value -match '^[^\s"]+$') { return $Value }
    $s = '"'; $slashCount = 0
    foreach ($ch in $Value.ToCharArray()) {
        if ($ch -eq '\') { $slashCount++; continue }
        if ($ch -eq '"') {
            $s += ('\' * ($slashCount * 2 + 1)) + '"'
            $slashCount = 0
            continue
        }
        if ($slashCount -gt 0) { $s += ('\' * $slashCount); $slashCount = 0 }
        $s += $ch
    }
    if ($slashCount -gt 0) { $s += ('\' * ($slashCount * 2)) }
    return ($s + '"')
}
function Get-LlamaServer([object]$Settings) {
    if (-not [string]::IsNullOrWhiteSpace([string]$Settings.llamaServer)) {
        return (Assert-File (Expand-LocalPath ([string]$Settings.llamaServer)) 'llama-server.exe')
    }
    $candidates = @(
        (Join-Path $ScriptRoot 'llama-server.exe'),
        (Join-Path $ScriptRoot 'llama.cpp\llama-server.exe'),
        (Join-Path $ScriptRoot 'llama.cpp\build\bin\Release\llama-server.exe')
    )
    foreach ($candidate in $candidates) {
        if ([IO.File]::Exists($candidate)) { return $candidate }
    }
    $cmd = Get-Command 'llama-server.exe' -ErrorAction SilentlyContinue
    if ($null -ne $cmd -and -not [string]::IsNullOrWhiteSpace($cmd.Source)) { return $cmd.Source }
    throw 'llama-server.exe not found. Set llamaServer in voice-servers.json (use a recent llama.cpp Windows CUDA build).'
}
function Get-GGUFInventory([object]$Settings) {
    $found = @{}
    foreach ($dirText in @($Settings.modelSearchDirs)) {
        $dir = Expand-LocalPath ([string]$dirText)
        if ([string]::IsNullOrWhiteSpace($dir) -or -not (Test-Path -LiteralPath $dir -PathType Container)) { continue }
        Write-Host "Searching GGUF: $dir"
        foreach ($file in @(Get-ChildItem -LiteralPath $dir -Filter '*.gguf' -File -Recurse -Force -ErrorAction SilentlyContinue)) {
            $found[$file.FullName.ToLowerInvariant()] = $file.FullName
        }
    }
    return @($found.Values)
}
function Select-UniqueFile([string[]]$Candidates, [string]$Description) {
    $files = @($Candidates | Sort-Object -Unique)
    if ($files.Count -eq 1) { return [string]$files[0] }
    if ($files.Count -eq 0) {
        throw "No $Description found. Specify its full path in voice-servers.json."
    }
    $list = ($files | ForEach-Object { "  $_" }) -join [Environment]::NewLine
    throw "Multiple $Description candidates were found. Choose one in voice-servers.json:`n$list"
}
function Resolve-Model([string]$Role, [object]$RoleConfig, [string[]]$Inventory) {
    if (-not [string]::IsNullOrWhiteSpace([string]$RoleConfig.model)) {
        $model = Assert-File (Expand-LocalPath ([string]$RoleConfig.model)) "$Role model GGUF"
    } else {
        $pattern = if ($Role -eq 'ASR') { 'qwen3[-_.]?asr' } else { 'qwen3[._-]?5' }
        $matches = @($Inventory | Where-Object {
            $name = [IO.Path]::GetFileName($_)
            $shard = [regex]::Match($name, '-(\d{5})-of-\d{5}\.gguf$', [Text.RegularExpressions.RegexOptions]::IgnoreCase)
            $firstShard = (-not $shard.Success) -or ($shard.Groups[1].Value -eq '00001')
            $_ -match $pattern -and $name -notmatch '(?i)mmproj|projector' -and $firstShard
        })
        $model = Select-UniqueFile $matches "$Role main GGUF"
    }
    if ([IO.Path]::GetExtension($model) -ine '.gguf') { throw "$Role model must be a .gguf: $model" }
    $mmproj = ''
    if (-not [string]::IsNullOrWhiteSpace([string]$RoleConfig.mmproj)) {
        $mmproj = Assert-File (Expand-LocalPath ([string]$RoleConfig.mmproj)) "$Role mmproj GGUF"
    } elseif ($Role -eq 'ASR') {
        # Prefer mmproj in the exact directory of the selected speech model.
        $projectors = @($Inventory | Where-Object {
            $name = [IO.Path]::GetFileName($_)
            $name -match 'mmproj' -and $_ -match 'qwen3[-_.]?asr'
        })
        $family = if ($model -match '(?i)0[.]6b') { '0.6b' } elseif ($model -match '(?i)1[.]7b') { '1.7b' } else { '' }
        if ($family) { $projectors = @($projectors | Where-Object { $_ -match [regex]::Escape($family) }) }
        $sameDir = @($projectors | Where-Object {
            [string]::Equals([IO.Path]::GetDirectoryName($_), [IO.Path]::GetDirectoryName($model), [StringComparison]::OrdinalIgnoreCase)
        })
        if ($sameDir.Count -gt 0) { $projectors = $sameDir }
        $mmproj = Select-UniqueFile $projectors 'ASR matching mmproj'
    }
    if ($Role -eq 'ASR' -and $model -match '(?i)qwen3[-_.]?asr' -and [string]::IsNullOrWhiteSpace($mmproj)) {
        throw 'Qwen3-ASR requires a matching audio mmproj GGUF.'
    }
    if ($Role -eq 'ASR' -and -not [string]::IsNullOrWhiteSpace($mmproj)) {
        # Never pair 1.7B ASR weights with a 0.6B projector (or vice versa).
        $modelSize = [regex]::Match($model, '(?i)(0[.]6|1[.]7)b')
        $projSize = [regex]::Match($mmproj, '(?i)(0[.]6|1[.]7)b')
        if ($modelSize.Success -and $projSize.Success -and
            $modelSize.Groups[1].Value -ne $projSize.Groups[1].Value) {
            throw 'ASR model and mmproj use different model sizes.'
        }
    }
    return @{ model = $model; mmproj = $mmproj }
}
function Make-Plan([string]$Role, [object]$Settings, [string[]]$Inventory, [string]$Exe) {
    $cfg = if ($Role -eq 'ASR') { $Settings.asr } else { $Settings.llm }
    if ($null -eq $cfg) { throw "Missing '$($Role.ToLowerInvariant())' section in voice-servers.json." }
    $port = [int]$cfg.port
    if ($port -lt 1 -or $port -gt 65535) { throw "$Role port out of range: $port" }
    $gpu = [int]$cfg.gpuLayers
    $context = [int]$cfg.ctxSize
    $parallel = [int]$cfg.parallel
    if ($gpu -lt 0 -or $context -lt 256 -or $parallel -lt 1) { throw "$Role invalid GPU layers, context size or parallel count." }
    $modelFiles = Resolve-Model $Role $cfg $Inventory
    $argsList = @('-m', $modelFiles.model, '-ngl', [string]$gpu, '-c', [string]$context,
                  '-np', [string]$parallel, '--host', '127.0.0.1', '--port', [string]$port,
                  '--no-webui')
    if (-not [string]::IsNullOrWhiteSpace($modelFiles.mmproj)) {
        $argsList += @('--mmproj', $modelFiles.mmproj)
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$cfg.alias)) {
        $argsList += @('--alias', [string]$cfg.alias)
    }
    if ($Role -eq 'LLM') {
        if ($cfg.jinja -eq $true) { $argsList += '--jinja' }
        # Match the LM Studio Qwen3.5-9B load settings shown in the screenshots.
        if ([int]$cfg.cpuThreads -gt 0) { $argsList += @('--threads', [string]$cfg.cpuThreads) }
        if ([int]$cfg.batchSize -gt 0) { $argsList += @('--batch-size', [string]$cfg.batchSize) }
        if ([int]$cfg.ubatchSize -gt 0) { $argsList += @('--ubatch-size', [string]$cfg.ubatchSize) }
        if ([int]$cfg.contextCheckpoints -ge 0) { $argsList += @('--ctx-checkpoints', [string]$cfg.contextCheckpoints) }
        if ($cfg.flashAttention -eq $true) { $argsList += @('--flash-attn', 'on') }
        if ($cfg.unifiedKV -eq $true) { $argsList += '--kv-unified' }
        if ($cfg.kvOffload -eq $true) { $argsList += '--kv-offload' }
        if ($cfg.tryMmap -eq $false) { $argsList += @('--load-mode', 'none') }
        if ($cfg.enableThinking -eq $false) { $argsList += @('--reasoning', 'off') }
    }
    if ($null -ne $cfg.extraArgs) {
        foreach ($arg in @($cfg.extraArgs)) { $argsList += [string]$arg }
    }
    return @{ role=$Role; exe=$Exe; model=$modelFiles.model; port=$port; args=$argsList }
}
function Test-ServiceEndpoint([string]$Role, [int]$Port) {
    $url = if ($Role -eq 'ASR') { "http://127.0.0.1:$Port/health" }
           else { "http://127.0.0.1:$Port/v1/models" }
    try {
        $response = Invoke-WebRequest -Uri $url -UseBasicParsing -Method Get -TimeoutSec 4 -ErrorAction Stop
        Write-Host "$Role API reachable: HTTP $($response.StatusCode) ($url)" -ForegroundColor Green
    } catch {
        $status = 'not ready'
        if ($null -ne $_.Exception.Response) {
            try { $status = "HTTP $([int]$_.Exception.Response.StatusCode)" } catch { }
        }
        Write-Warning "$Role API $status at $url. Check logs; this probe does NOT test real ASR/transcription or chat."
    }
}

if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
    throw "Config file not found: $ConfigPath"
}
$Settings = Get-Content -LiteralPath $ConfigPath -Encoding UTF8 -Raw | ConvertFrom-Json
$State = Read-State

switch ($Action) {
    'Stop' {
        foreach ($role in $SelectedRoles) {
            $proc = Get-RecordedProcess $role $State
            if ($null -ne $proc) {
                Stop-Process -Id ([int]$proc.ProcessId) -ErrorAction Stop
                Write-Host "Stopped $role (PID $($proc.ProcessId))." -ForegroundColor Green
                $State.Remove($role)
            } else {
                Write-Host "${role}: no verified script-owned process to stop. Leaving all other processes untouched."
                if ($State.ContainsKey($role)) { $State.Remove($role) }
            }
        }
        Save-State $State
        break
    }
    'Status' {
        foreach ($role in $SelectedRoles) {
            $cfg = if ($role -eq 'ASR') { $Settings.asr } else { $Settings.llm }
            $port = [int]$cfg.port
            $proc = Get-RecordedProcess $role $State
            $owned = if ($null -ne $proc) { "script PID $($proc.ProcessId)" } else { 'not script-owned' }
            $listener = if (Test-LocalPort $port) { 'listening' } else { 'not listening' }
            Write-Host "$role : 127.0.0.1:$port / $listener / $owned"
        }
        break
    }
    'Test' {
        foreach ($role in $SelectedRoles) {
            $cfg = if ($role -eq 'ASR') { $Settings.asr } else { $Settings.llm }
            Test-ServiceEndpoint $role ([int]$cfg.port)
        }
        break
    }
    'Start' {
        $exe = Get-LlamaServer $Settings
        $inventory = Get-GGUFInventory $Settings
        $plans = @()
        foreach ($role in $SelectedRoles) { $plans += ,(Make-Plan $role $Settings $inventory $exe) }
        if ($plans.Count -gt 1 -and $plans[0].port -eq $plans[1].port) {
            throw 'ASR and LLM must use distinct ports.'
        }
        foreach ($plan in $plans) {
            $role = [string]$plan.role
            $cmdLine = @($plan.args | ForEach-Object { Quote-NativeArg ([string]$_) }) -join ' '
            Write-Host "`n[$role] $(Quote-NativeArg $plan.exe) $cmdLine"
            if ($DryRun) { continue }
            $proc = Get-RecordedProcess $role $State
            if ($null -ne $proc) {
                Write-Host "$role already managed (PID $($proc.ProcessId)); not starting a duplicate."
                continue
            }
            if (Test-LocalPort ([int]$plan.port)) {
                throw "$role port $($plan.port) is already occupied. Stop the existing LM Studio/llama-server API or change this port in voice-servers.json. No existing process was terminated."
            }
            if (-not (Test-Path -LiteralPath $RuntimeDir)) {
                $null = New-Item -ItemType Directory -Path $RuntimeDir -Force
            }
            $outLog = Join-Path $RuntimeDir "$($role.ToLowerInvariant()).stdout.log"
            $errLog = Join-Path $RuntimeDir "$($role.ToLowerInvariant()).stderr.log"
            # Start-Process accepts a single fully quoted argument string. This
            # preserves paths containing spaces (including LM Studio model paths).
            $process = Start-Process -FilePath $plan.exe -WorkingDirectory ([IO.Path]::GetDirectoryName($plan.exe)) `
                -ArgumentList $cmdLine -PassThru -WindowStyle Hidden `
                -RedirectStandardOutput $outLog -RedirectStandardError $errLog -ErrorAction Stop
            $State[$role] = @{
                pid = [int]$process.Id; server = [string]$plan.exe; model = [string]$plan.model
                port = [int]$plan.port; startedUtc = (Get-Date).ToUniversalTime().ToString('o')
            }
            Save-State $State
            Start-Sleep -Milliseconds 250
            $process.Refresh()
            if ($process.HasExited) {
                Write-Warning "$role process exited immediately, code $($process.ExitCode). See $errLog"
            } else {
                Write-Host "$role launched, PID $($process.Id). API: http://127.0.0.1:$($plan.port)/v1" -ForegroundColor Green
                Write-Host "Logs: $errLog"
            }
        }
        if ($DryRun) { Write-Host '`nDry-run only: no processes launched.' }
        else { Write-Host "`nUse -Action Status / -Action Test to inspect the servers after model loading." }
        break
    }
}
