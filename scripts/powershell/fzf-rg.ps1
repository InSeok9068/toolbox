[CmdletBinding()]
param()

. (Join-Path $PSScriptRoot 'toolbox.common.ps1')

Set-ToolboxUtf8Console

$rgPath = Get-ToolboxBundledCommand -Name 'rg.exe'
$fzfPath = Get-ToolboxBundledCommand -Name 'fzf.exe'
$batPath = Get-ToolboxBundledCommand -Name 'bat.exe'
$codePath = Get-ToolboxSystemCommand -Name 'code'

$excludeGlobs = @(
    '!.git',
    '!node_modules',
    '!dist',
    '!build',
    '!.next',
    '!coverage'
)

$rgPrefixParts = @(
    (ConvertTo-ToolboxShellArgument -Value $rgPath),
    '--column',
    '--line-number',
    '--no-heading',
    '--color=always',
    '--smart-case',
    '--hidden'
) + ($excludeGlobs | ForEach-Object { "-g $_" })

$rgPrefix = $rgPrefixParts -join ' '
$startReload = "start:reload:$rgPrefix '' || cd ."
$changeReload = "change:reload:$rgPrefix {q} || cd ."
$previewCommand = "$(ConvertTo-ToolboxShellArgument -Value $batPath) --color=always --style=numbers --line-range=:500 --highlight-line {2} -- {1}"

& $fzfPath `
    --ansi `
    --disabled `
    --prompt 'rg> ' `
    --delimiter ':' `
    --with-nth '1,2,4..' `
    --bind $startReload `
    --bind $changeReload `
    --preview $previewCommand `
    --preview-window 'right:60%:wrap,+{2}/2' |
    ForEach-Object {
        if ($_ -match '^(.*?):([0-9]+):([0-9]+):(.*)$') {
            & $codePath --goto "$($matches[1]):$($matches[2]):$($matches[3])"
        }
    }
