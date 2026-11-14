#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

# Personal paths belong in the gitignored file next to this script, not in the repository.
$localPaths = Join-Path $PSScriptRoot 'local-paths.ps1'
if (-not (Test-Path -LiteralPath $localPaths -PathType Leaf)) {
    throw 'Missing local-paths.ps1. Copy local-paths.example.ps1 to local-paths.ps1 and set your local paths.'
}
. $localPaths
foreach ($name in @('llamaServer', 'asrModel', 'asrMmproj', 'llmModel')) {
    $value = Get-Variable -Name $name -ValueOnly -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace([string]$value)) {
        throw "Set variable $name in local-paths.ps1 before starting the models."
    }
}

foreach ($file in @($llamaServer, $asrModel, $asrMmproj, $llmModel)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "File not found: $file" }
}

# Qwen3-ASR endpoint: http://127.0.0.1:8080/v1
$asrArgs = '-m "{0}" --mmproj "{1}" --host 127.0.0.1 --port 8080 -ngl 99 -c 8192 -np 1 --alias whisper-1 --no-webui' -f $asrModel, $asrMmproj

# Qwen3.5 endpoint: http://127.0.0.1:1234/v1
# Adjust model parameters for the installed llama.cpp build and GPU.
$llmArgs = '-m "{0}" --host 127.0.0.1 --port 1234 -ngl 32 -c 65536 -np 1 --threads 6 --batch-size 2048 --ubatch-size 512 --ctx-checkpoints 32 --flash-attn on --kv-unified --kv-offload --load-mode none --jinja --reasoning off --alias qwen3.5 --no-webui' -f $llmModel

$workDir = Split-Path -Parent $llamaServer
Start-Process -FilePath $llamaServer -WorkingDirectory $workDir -ArgumentList $asrArgs
Start-Process -FilePath $llamaServer -WorkingDirectory $workDir -ArgumentList $llmArgs
Write-Host 'ASR: http://127.0.0.1:8080/v1'
Write-Host 'LLM: http://127.0.0.1:1234/v1'
