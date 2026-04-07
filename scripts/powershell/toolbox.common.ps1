Set-StrictMode -Version Latest

function Set-ToolboxUtf8Console {
    $utf8 = [System.Text.UTF8Encoding]::new()

    [Console]::InputEncoding = $utf8
    [Console]::OutputEncoding = $utf8
    $global:OutputEncoding = $utf8

    chcp 65001 | Out-Null
}

function Assert-ToolboxCommand {
    param(
        [Parameter(Mandatory)]
        [string[]]$Name
    )

    foreach ($commandName in $Name) {
        if (-not (Get-Command $commandName -ErrorAction SilentlyContinue)) {
            throw "Required command not found: $commandName"
        }
    }
}
