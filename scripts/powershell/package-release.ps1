[CmdletBinding()]
param(
    [string]$OutputDirectory = (Join-Path (Join-Path $PSScriptRoot '..\..') 'dist'),
    [string]$ArtifactName = 'toolbox-windows-amd64'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$outputDirectoryPath = [System.IO.Path]::GetFullPath($OutputDirectory)
$stageDirectoryPath = [System.IO.Path]::GetFullPath((Join-Path $outputDirectoryPath $ArtifactName))
$zipPath = Join-Path $outputDirectoryPath "$ArtifactName.zip"
$bundledBinDirectory = Join-Path $repoRoot 'bin'
$toolboxPath = Join-Path $stageDirectoryPath 'toolbox.exe'

if (-not $stageDirectoryPath.StartsWith($outputDirectoryPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Stage directory must stay inside the output directory: $stageDirectoryPath"
}

if (-not (Test-Path -LiteralPath $bundledBinDirectory -PathType Container)) {
    throw "Bundled bin directory not found: $bundledBinDirectory"
}

New-Item -ItemType Directory -Path $outputDirectoryPath -Force | Out-Null

if (Test-Path -LiteralPath $stageDirectoryPath) {
    Remove-Item -LiteralPath $stageDirectoryPath -Recurse -Force
}

if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}

New-Item -ItemType Directory -Path $stageDirectoryPath -Force | Out-Null

go build -o $toolboxPath ./cmd
if ($LASTEXITCODE -ne 0) {
    throw 'go build failed'
}

Copy-Item -LiteralPath $bundledBinDirectory -Destination (Join-Path $stageDirectoryPath 'bin') -Recurse

Compress-Archive -Path (Join-Path $stageDirectoryPath '*') -DestinationPath $zipPath

Write-Host "Created release zip: $zipPath"
