[CmdletBinding(SupportsShouldProcess)]
param()

. (Join-Path $PSScriptRoot 'toolbox.common.ps1')

Set-ToolboxUtf8Console
Assert-ToolboxCommand -Name @('scoop', 'git')

$requiredBucket = 'extras'
$packages = @(
    'delta',
    'lazygit',
    'ripgrep',
    'fd',
    'fzf',
    'bat'
)

function Test-ScoopBucketInstalled {
    param(
        [Parameter(Mandatory)]
        [string]$Name
    )

    $bucketList = scoop bucket list | Out-String
    return $bucketList -match "(?m)^\s*$([regex]::Escape($Name))\s"
}

function Test-ScoopPackageInstalled {
    param(
        [Parameter(Mandatory)]
        [string]$Name
    )

    scoop prefix $Name *> $null
    return $LASTEXITCODE -eq 0
}

function Invoke-ScoopInstall {
    param(
        [Parameter(Mandatory)]
        [string[]]$Argument
    )

    & scoop @Argument

    if ($LASTEXITCODE -ne 0) {
        throw "scoop command failed: scoop $($Argument -join ' ')"
    }
}

if (-not (Test-ScoopBucketInstalled -Name $requiredBucket)) {
    Write-Host "Adding scoop bucket: $requiredBucket"

    if ($PSCmdlet.ShouldProcess("scoop bucket $requiredBucket", 'add')) {
        Invoke-ScoopInstall -Argument @('bucket', 'add', $requiredBucket)
    }
} else {
    Write-Host "Bucket already installed: $requiredBucket"
}

foreach ($package in $packages) {
    if (Test-ScoopPackageInstalled -Name $package) {
        Write-Host "Already installed: $package"
        continue
    }

    Write-Host "Installing package: $package"

    if ($PSCmdlet.ShouldProcess("scoop package $package", 'install')) {
        Invoke-ScoopInstall -Argument @('install', $package)
    }
}

Write-Host 'Dependency installation check complete.'
