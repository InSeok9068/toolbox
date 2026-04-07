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

function Get-ToolboxBundledCommand {
    param(
        [Parameter(Mandatory)]
        [string]$Name
    )

    if ([string]::IsNullOrWhiteSpace($env:TOOLBOX_BIN_DIR)) {
        throw 'TOOLBOX_BIN_DIR is not set.'
    }

    $commandPath = Join-Path $env:TOOLBOX_BIN_DIR $Name
    if (-not (Test-Path -LiteralPath $commandPath -PathType Leaf)) {
        throw "Bundled command not found: $commandPath"
    }

    return $commandPath
}

function Get-ToolboxSystemCommand {
    param(
        [Parameter(Mandatory)]
        [string]$Name
    )

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "Required command not found: $Name"
    }

    return $command.Source
}

function ConvertTo-ToolboxShellArgument {
    param(
        [Parameter(Mandatory)]
        [string]$Value
    )

    $normalizedValue = $Value
    if ($env:OS -eq 'Windows_NT') {
        $normalizedValue = $normalizedValue -replace '\\', '/'
    }

    return "'" + $normalizedValue.Replace("'", "'\''") + "'"
}
