[CmdletBinding()]
param(
    [ValidatePattern('^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$')]
    [string]$Version = 'v1.3.7',

    [string]$InstallDir = $(
        if ($env:LOCALAPPDATA) {
            Join-Path $env:LOCALAPPDATA 'Programs\mss\bin'
        } else {
            Join-Path $HOME '.local\bin'
        }
    )
)

$ErrorActionPreference = 'Stop'
$Repository = 'mss-boot-io/mss-boot-admin'

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    throw 'InstallDir must not be empty.'
}
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'install-mss.ps1 supports Windows only.'
}

$Architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported Windows architecture: $($_)" }
}

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$Asset = "mss-tools-$Version-windows-$Architecture.zip"
$Manifest = "SHA256SUMS.tools-$Version"
$BaseUrl = "https://github.com/$Repository/releases/download/$Version"
$TemporaryDir = Join-Path ([System.IO.Path]::GetTempPath()) ("mss-install-" + [guid]::NewGuid().ToString('N'))
$StageMss = $null
$StageMcp = $null
$BackupMss = $null
$BackupMcp = $null

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
    New-Item -ItemType Directory -Path $TemporaryDir | Out-Null
    $ManifestPath = Join-Path $TemporaryDir $Manifest
    $AssetPath = Join-Path $TemporaryDir $Asset
    Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/$Manifest" -OutFile $ManifestPath
    Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/$Asset" -OutFile $AssetPath

    $MatchesForAsset = @()
    foreach ($Line in Get-Content -LiteralPath $ManifestPath) {
        if ($Line -match '^([0-9a-fA-F]{64})\s+\*?([^\s]+)$' -and $Matches[2] -eq $Asset) {
            $MatchesForAsset += $Matches[1].ToLowerInvariant()
        }
    }
    if ($MatchesForAsset.Count -ne 1) {
        throw "Checksum manifest must contain exactly one valid entry for $Asset."
    }
    $ActualHash = Get-Sha256Hex -Path $AssetPath
    if ($ActualHash -ne $MatchesForAsset[0]) {
        throw "Checksum verification failed for $Asset."
    }

    $Unpacked = Join-Path $TemporaryDir 'unpacked'
    Expand-Archive -LiteralPath $AssetPath -DestinationPath $Unpacked
    $ExpectedFiles = @('BUILD-INFO', 'LICENSE', 'mss.exe', 'mss-mcp.exe')
    $ActualFiles = @(Get-ChildItem -LiteralPath $Unpacked -File -Recurse | ForEach-Object {
        $_.FullName.Substring($Unpacked.Length).TrimStart('\', '/').Replace('\', '/')
    } | Sort-Object)
    if (($ActualFiles -join "`n") -ne (($ExpectedFiles | Sort-Object) -join "`n")) {
        throw 'Tool archive has an unexpected file set.'
    }

    $BuildInfo = @{}
    foreach ($Line in Get-Content -LiteralPath (Join-Path $Unpacked 'BUILD-INFO')) {
        $Pair = $Line -split '=', 2
        if ($Pair.Count -eq 2) { $BuildInfo[$Pair[0]] = $Pair[1] }
    }
    $ParsedTimestamp = [DateTimeOffset]::MinValue
    $TimestampIsValid = [DateTimeOffset]::TryParse(
        $BuildInfo.timestamp,
        [System.Globalization.CultureInfo]::InvariantCulture,
        [System.Globalization.DateTimeStyles]::RoundtripKind,
        [ref]$ParsedTimestamp
    )
    if ($BuildInfo.version -ne $Version -or $BuildInfo.commit -notmatch '^[0-9a-f]{40}$' -or -not $TimestampIsValid) {
        throw 'Tool archive BUILD-INFO has invalid release provenance.'
    }
    foreach ($CommandName in @('mss.exe', 'mss-mcp.exe')) {
        $Output = (& (Join-Path $Unpacked $CommandName) --version 2>&1 | Out-String)
        if (-not $Output.Contains($Version) -or -not $Output.Contains($BuildInfo.commit) -or -not $Output.Contains($BuildInfo.timestamp)) {
            throw "$CommandName version output does not match BUILD-INFO."
        }
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $DestinationMss = Join-Path $InstallDir 'mss.exe'
    $DestinationMcp = Join-Path $InstallDir 'mss-mcp.exe'
    $StageMss = Join-Path $InstallDir ('.mss.install.' + [guid]::NewGuid().ToString('N'))
    $StageMcp = Join-Path $InstallDir ('.mss-mcp.install.' + [guid]::NewGuid().ToString('N'))
    $BackupMss = Join-Path $InstallDir ('.mss.backup.' + [guid]::NewGuid().ToString('N'))
    $BackupMcp = Join-Path $InstallDir ('.mss-mcp.backup.' + [guid]::NewGuid().ToString('N'))
    Copy-Item -LiteralPath (Join-Path $Unpacked 'mss.exe') -Destination $StageMss
    Copy-Item -LiteralPath (Join-Path $Unpacked 'mss-mcp.exe') -Destination $StageMcp
    $HadMss = Test-Path -LiteralPath $DestinationMss -PathType Leaf
    $HadMcp = Test-Path -LiteralPath $DestinationMcp -PathType Leaf
    if ($HadMss) { Copy-Item -LiteralPath $DestinationMss -Destination $BackupMss }
    if ($HadMcp) { Copy-Item -LiteralPath $DestinationMcp -Destination $BackupMcp }

    $ReplacedMss = $false
    $ReplacedMcp = $false
    try {
        Move-Item -LiteralPath $StageMss -Destination $DestinationMss -Force
        $StageMss = $null
        $ReplacedMss = $true
        Move-Item -LiteralPath $StageMcp -Destination $DestinationMcp -Force
        $StageMcp = $null
        $ReplacedMcp = $true

        foreach ($Installed in @(
            @{ Name = 'mss.exe'; Path = $DestinationMss },
            @{ Name = 'mss-mcp.exe'; Path = $DestinationMcp }
        )) {
            $Output = (& $Installed.Path --version 2>&1 | Out-String)
            if (
                -not $Output.Contains($Version) -or
                -not $Output.Contains($BuildInfo.commit) -or
                -not $Output.Contains($BuildInfo.timestamp)
            ) {
                throw "$($Installed.Name) installed identity does not match BUILD-INFO."
            }
        }
    }
    catch {
        $InstallFailure = $_
        $RollbackFailures = @()
        if ($ReplacedMss) {
            try {
                if ($HadMss) {
                    Move-Item -LiteralPath $BackupMss -Destination $DestinationMss -Force
                    $BackupMss = $null
                } else {
                    Remove-Item -LiteralPath $DestinationMss -Force
                }
            }
            catch { $RollbackFailures += 'mss.exe' }
        }
        if ($ReplacedMcp) {
            try {
                if ($HadMcp) {
                    Move-Item -LiteralPath $BackupMcp -Destination $DestinationMcp -Force
                    $BackupMcp = $null
                } else {
                    Remove-Item -LiteralPath $DestinationMcp -Force
                }
            }
            catch { $RollbackFailures += 'mss-mcp.exe' }
        }
        if ($RollbackFailures.Count -gt 0) {
            throw "Tool installation failed and rollback also failed for: $($RollbackFailures -join ', '). Original error: $InstallFailure"
        }
        throw $InstallFailure
    }

    if ($HadMss) { Remove-Item -LiteralPath $BackupMss -Force }
    if ($HadMcp) { Remove-Item -LiteralPath $BackupMcp -Force }
    $BackupMss = $null
    $BackupMcp = $null

    Write-Output "Installed mss and mss-mcp $Version to $InstallDir"
    Write-Output 'Add this directory to PATH yourself if needed; no shell profile was changed.'
}
finally {
    if ($StageMss -and (Test-Path -LiteralPath $StageMss)) { Remove-Item -LiteralPath $StageMss -Force }
    if ($StageMcp -and (Test-Path -LiteralPath $StageMcp)) { Remove-Item -LiteralPath $StageMcp -Force }
    if ($BackupMss -and (Test-Path -LiteralPath $BackupMss)) { Remove-Item -LiteralPath $BackupMss -Force }
    if ($BackupMcp -and (Test-Path -LiteralPath $BackupMcp)) { Remove-Item -LiteralPath $BackupMcp -Force }
    if (Test-Path -LiteralPath $TemporaryDir) { Remove-Item -LiteralPath $TemporaryDir -Recurse -Force }
}
