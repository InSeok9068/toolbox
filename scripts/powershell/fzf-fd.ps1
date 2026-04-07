[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot 'toolbox.common.ps1')

Set-ToolboxUtf8Console

$fdPath = Get-ToolboxBundledCommand -Name 'fd.exe'
$fzfPath = Get-ToolboxBundledCommand -Name 'fzf.exe'
$batPath = Get-ToolboxBundledCommand -Name 'bat.exe'
$codePath = Get-ToolboxSystemCommand -Name 'code'

$fdArguments = @(
    '--type', 'f',
    '--hidden',
    '--exclude', '.git',
    '--exclude', 'node_modules',
    '--exclude', 'dist',
    '--exclude', 'build',
    '--exclude', '.next',
    '--exclude', 'coverage'
)

$previewCommand = "$(ConvertTo-ToolboxShellArgument -Value $batPath) --color=always --style=numbers --line-range=:500 {}"

& $fdPath @fdArguments |
    & $fzfPath `
        --preview $previewCommand `
        --preview-window 'right:60%:wrap' `
        --bind 'ctrl-/:toggle-preview' |
    ForEach-Object {
        if (-not [string]::IsNullOrWhiteSpace($_)) {
            & $codePath $_
        }
    }
