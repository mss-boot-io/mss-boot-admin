[CmdletBinding()]
param(
    [string]$MssFixture,
    [string]$McpFixture,
    [string]$CandidateCommit,
    [string]$CandidateTimestamp
)

$ErrorActionPreference = 'Stop'

if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'test-install-mss.ps1 requires Windows.'
}

$RepositoryRoot = [System.IO.Path]::GetFullPath(
    (Resolve-Path (Join-Path $PSScriptRoot '..\..')).ProviderPath
)
$GitSafeRoot = $RepositoryRoot.Replace('\', '/')
$Installer = Join-Path $PSScriptRoot 'install-mss.ps1'
$Version = 'v1.3.5'
$UsePrebuilt = -not [string]::IsNullOrWhiteSpace($MssFixture) -or -not [string]::IsNullOrWhiteSpace($McpFixture)
if ($UsePrebuilt) {
    if (
        [string]::IsNullOrWhiteSpace($MssFixture) -or
        [string]::IsNullOrWhiteSpace($McpFixture) -or
        [string]::IsNullOrWhiteSpace($CandidateCommit) -or
        [string]::IsNullOrWhiteSpace($CandidateTimestamp)
    ) {
        throw 'Prebuilt fixtures require both executables plus CandidateCommit and CandidateTimestamp.'
    }
    $Commit = $CandidateCommit.Trim()
    $Timestamp = $CandidateTimestamp.Trim()
} else {
    $Commit = (& git -c "safe.directory=$GitSafeRoot" -C $RepositoryRoot rev-parse HEAD).Trim()
    $Timestamp = (& git -c "safe.directory=$GitSafeRoot" -C $RepositoryRoot show --no-show-signature -s --format=%cI HEAD).Trim()
}
if ($Commit -notmatch '^[0-9a-f]{40}$' -or [string]::IsNullOrWhiteSpace($Timestamp)) {
    throw 'Cannot resolve candidate release provenance.'
}

$Architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported Windows test architecture: $($_)" }
}
$TestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('mss-installer-test-' + [guid]::NewGuid().ToString('N'))
$Fixture = Join-Path $TestRoot 'fixture'
$ArchiveRoot = Join-Path $TestRoot 'archive'
$InstallDir = Join-Path $TestRoot 'installed'
$Asset = "mss-tools-$Version-windows-$Architecture.zip"
$Manifest = "SHA256SUMS.tools-$Version"
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$OriginalGoWork = $env:GOWORK

function Get-Sha256Hex {
    param([Parameter(Mandatory = $true)][string]$Path)
    $Stream = [System.IO.File]::OpenRead($Path)
    $Algorithm = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($Algorithm.ComputeHash($Stream))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $Algorithm.Dispose()
        $Stream.Dispose()
    }
}

try {
    New-Item -ItemType Directory -Path $Fixture, $ArchiveRoot, $InstallDir | Out-Null
    $Ldflags = "-s -w -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Version=$Version -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Commit=$Commit -X github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo.Timestamp=$Timestamp"
    if ($UsePrebuilt) {
        Copy-Item -LiteralPath $MssFixture -Destination (Join-Path $ArchiveRoot 'mss.exe')
        Copy-Item -LiteralPath $McpFixture -Destination (Join-Path $ArchiveRoot 'mss-mcp.exe')
    } else {
        $env:GOWORK = 'off'
        & go build -trimpath -ldflags $Ldflags -o (Join-Path $ArchiveRoot 'mss.exe') (Join-Path $RepositoryRoot 'cmd\mss')
        if ($LASTEXITCODE -ne 0) { throw 'Cannot build the Windows mss fixture.' }
        & go build -trimpath -ldflags $Ldflags -o (Join-Path $ArchiveRoot 'mss-mcp.exe') (Join-Path $RepositoryRoot 'cmd\mss-mcp')
        if ($LASTEXITCODE -ne 0) { throw 'Cannot build the Windows mss-mcp fixture.' }
    }
    Copy-Item -LiteralPath (Join-Path $RepositoryRoot 'LICENSE') -Destination (Join-Path $ArchiveRoot 'LICENSE')
    [System.IO.File]::WriteAllText(
        (Join-Path $ArchiveRoot 'BUILD-INFO'),
        "version=$Version`ncommit=$Commit`ntimestamp=$Timestamp`n",
        $Utf8NoBom
    )
    Compress-Archive -LiteralPath @(
        (Join-Path $ArchiveRoot 'BUILD-INFO'),
        (Join-Path $ArchiveRoot 'LICENSE'),
        (Join-Path $ArchiveRoot 'mss.exe'),
        (Join-Path $ArchiveRoot 'mss-mcp.exe')
    ) -DestinationPath (Join-Path $Fixture $Asset)
    $Hash = Get-Sha256Hex -Path (Join-Path $Fixture $Asset)
    [System.IO.File]::WriteAllText(
        (Join-Path $Fixture $Manifest),
        "$Hash  $Asset`n",
        $Utf8NoBom
    )

    function global:Invoke-WebRequest {
        param(
            [switch]$UseBasicParsing,
            [Parameter(Mandatory = $true)][string]$Uri,
            [Parameter(Mandatory = $true)][string]$OutFile
        )
        $Name = [System.IO.Path]::GetFileName(([Uri]$Uri).AbsolutePath)
        Copy-Item -LiteralPath (Join-Path $Fixture $Name) -Destination $OutFile
    }

    & $Installer -Version $Version -InstallDir $InstallDir
    foreach ($CommandName in @('mss.exe', 'mss-mcp.exe')) {
        $Path = Join-Path $InstallDir $CommandName
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
            throw "$CommandName was not installed."
        }
        $Output = (& $Path --version 2>&1 | Out-String)
        if (-not $Output.Contains($Version) -or -not $Output.Contains($Commit)) {
            throw "$CommandName has the wrong installed identity."
        }
    }

    [System.IO.File]::AppendAllText((Join-Path $InstallDir 'mss.exe'), 'old-mss-marker', $Utf8NoBom)
    [System.IO.File]::AppendAllText((Join-Path $InstallDir 'mss-mcp.exe'), 'old-mcp-marker', $Utf8NoBom)
    $BeforeAtomicMss = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss.exe')
    $BeforeAtomicMcp = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss-mcp.exe')
    $global:MssInstallerFailureMarker = Join-Path $TestRoot 'second-move-failed'
    function global:Move-Item {
        [CmdletBinding()]
        param(
            [Parameter(Mandatory = $true)][string]$LiteralPath,
            [Parameter(Mandatory = $true)][string]$Destination,
            [switch]$Force
        )
        if (
            $LiteralPath -like '*.mss-mcp.install.*' -and
            $Destination -like '*mss-mcp.exe' -and
            -not (Test-Path -LiteralPath $global:MssInstallerFailureMarker)
        ) {
            [System.IO.File]::WriteAllText($global:MssInstallerFailureMarker, 'failed')
            throw 'injected second replacement failure'
        }
        Microsoft.PowerShell.Management\Move-Item @PSBoundParameters
    }
    $RejectedPartialReplacement = $false
    try {
        & $Installer -Version $Version -InstallDir $InstallDir *> $null
    }
    catch {
        $RejectedPartialReplacement = $true
    }
    finally {
        if (Test-Path Function:\global:Move-Item) {
            Remove-Item Function:\global:Move-Item -Force
        }
    }
    if (-not $RejectedPartialReplacement) {
        throw 'The installer succeeded after the second replacement was forced to fail.'
    }
    if (-not (Test-Path -LiteralPath $global:MssInstallerFailureMarker)) {
        throw 'The injected second replacement failure did not run.'
    }
    $AfterAtomicMss = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss.exe')
    $AfterAtomicMcp = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss-mcp.exe')
    if ($BeforeAtomicMss -ne $AfterAtomicMss -or $BeforeAtomicMcp -ne $AfterAtomicMcp) {
        throw 'A failed second replacement left a mixed Windows tool set.'
    }

    $Before = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss.exe')
    [System.IO.File]::AppendAllText((Join-Path $Fixture $Asset), 'tamper', $Utf8NoBom)
    $RejectedTamper = $false
    try {
        & $Installer -Version $Version -InstallDir $InstallDir *> $null
    }
    catch {
        $RejectedTamper = $true
    }
    if (-not $RejectedTamper) { throw 'The installer accepted a tampered Windows archive.' }
    $After = Get-Sha256Hex -Path (Join-Path $InstallDir 'mss.exe')
    if ($Before -ne $After) { throw 'A failed install changed the existing mss.exe.' }

    $RejectedMovingVersion = $false
    try {
        & $Installer -Version 'latest' -InstallDir $InstallDir *> $null
    }
    catch {
        $RejectedMovingVersion = $true
    }
    if (-not $RejectedMovingVersion) { throw 'The installer accepted a moving version.' }

    Write-Output 'PASS Windows install-mss checksum, provenance, atomic rollback, replacement, and tamper rejection'
}
finally {
    $env:GOWORK = $OriginalGoWork
    if (Test-Path Function:\global:Invoke-WebRequest) {
        Remove-Item Function:\global:Invoke-WebRequest -Force
    }
    if (Test-Path Function:\global:Move-Item) {
        Remove-Item Function:\global:Move-Item -Force
    }
    Remove-Variable -Name MssInstallerFailureMarker -Scope Global -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $TestRoot) {
        Remove-Item -LiteralPath $TestRoot -Recurse -Force
    }
}
