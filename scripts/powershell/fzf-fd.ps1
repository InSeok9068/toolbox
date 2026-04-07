[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot 'toolbox.common.ps1')

Set-ToolboxUtf8Console
Assert-ToolboxCommand -Name @('fd', 'fzf', 'bat', 'code')

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

$previewCommand = 'bat --color=always --style=numbers --line-range=:500 {}'

& fd @fdArguments |
    fzf `
        --preview $previewCommand `
        --preview-window 'right:60%:wrap' `
        --bind 'ctrl-/:toggle-preview' |
    ForEach-Object {
        if (-not [string]::IsNullOrWhiteSpace($_)) {
            code $_
        }
    }
