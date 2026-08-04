$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $repoRoot "dist\npm"
$appFile = Join-Path $repoRoot "internal\app\app.go"
$appContent = Get-Content -LiteralPath $appFile -Raw

if ($appContent -notmatch 'Version\s*=\s*"([^"]+)"') {
    throw "Cannot read app version from internal\app\app.go"
}

$version = $Matches[1]

$packagePaths = @(
    "npm\packages\win32-x64\package.json",
    "npm\packages\linux-x64\package.json",
    "npm\packages\darwin-arm64\package.json",
    "npm\onesearch\package.json",
    "npm\deqiying-onesearch\package.json"
)

foreach ($packagePath in $packagePaths) {
    $fullPath = Join-Path $repoRoot $packagePath
    $package = Get-Content -LiteralPath $fullPath -Raw | ConvertFrom-Json
    if ($package.version -ne $version) {
        throw "$packagePath version $($package.version) does not match app version $version"
    }
}

$targets = @(
    @{
        PackageDir = "npm\packages\win32-x64"
        GOOS = "windows"
        GOARCH = "amd64"
        BinaryName = "onesearch.exe"
    },
    @{
        PackageDir = "npm\packages\linux-x64"
        GOOS = "linux"
        GOARCH = "amd64"
        BinaryName = "onesearch"
    },
    @{
        PackageDir = "npm\packages\darwin-arm64"
        GOOS = "darwin"
        GOARCH = "arm64"
        BinaryName = "onesearch"
    }
)

New-Item -ItemType Directory -Force -Path $distDir | Out-Null

$oldCGO = $env:CGO_ENABLED
$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldGOCACHE = $env:GOCACHE
$oldGOMODCACHE = $env:GOMODCACHE
$oldGOTELEMETRY = $env:GOTELEMETRY
$oldNpmCache = $env:npm_config_cache

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath failed with exit code $LASTEXITCODE"
    }
}

function Invoke-NpmPack {
    param(
        [Parameter(Mandatory = $true)]
        [string]$PackageDir
    )

    $npmCacheRoot = Join-Path $distDir "npm-cache"

    for ($attempt = 1; $attempt -le 3; $attempt++) {
        $env:npm_config_cache = Join-Path $npmCacheRoot ([guid]::NewGuid().ToString("N"))
        New-Item -ItemType Directory -Force -Path $env:npm_config_cache | Out-Null

        try {
            Push-Location (Join-Path $repoRoot $PackageDir)
            try {
                Invoke-Checked -FilePath "npm" -Arguments @("pack", "--pack-destination", $distDir)
                return
            }
            finally {
                Pop-Location
            }
        }
        catch {
            if ($attempt -eq 3) {
                throw
            }

            Start-Sleep -Seconds 1
        }
    }
}

try {
    $env:GOCACHE = Join-Path $repoRoot ".gocache"
    $env:GOMODCACHE = Join-Path $repoRoot ".gomodcache"
    $env:GOTELEMETRY = "off"

    foreach ($target in $targets) {
        $binDir = Join-Path $repoRoot (Join-Path $target.PackageDir "bin")
        New-Item -ItemType Directory -Force -Path $binDir | Out-Null

        $env:CGO_ENABLED = "0"
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH

        $outputPath = Join-Path $binDir $target.BinaryName
        Invoke-Checked -FilePath "go" -Arguments @("build", "-trimpath", "-ldflags=-s -w", "-o", $outputPath, ".\cmd\onesearch")
    }
}
finally {
    $env:CGO_ENABLED = $oldCGO
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
    $env:GOCACHE = $oldGOCACHE
    $env:GOMODCACHE = $oldGOMODCACHE
    $env:GOTELEMETRY = $oldGOTELEMETRY
    $env:npm_config_cache = $oldNpmCache
}

$windowsBinary = Join-Path $repoRoot "npm\packages\win32-x64\bin\onesearch.exe"
& $windowsBinary --version
if ($LASTEXITCODE -ne 0) {
    throw "$windowsBinary --version failed with exit code $LASTEXITCODE"
}

$schemaJson = & $windowsBinary schema skills show --format json
if ($LASTEXITCODE -ne 0) {
    throw "$windowsBinary schema skills show failed with exit code $LASTEXITCODE"
}
$schema = $schemaJson | ConvertFrom-Json
if ($schema.ok -ne $true -or @($schema.commands).Count -ne 1 -or $schema.commands[0].id -ne "skills.show") {
    throw "Targeted skills.show schema is missing the expected contract"
}

$skillsJson = & $windowsBinary skills list --format json
if ($LASTEXITCODE -ne 0) {
    throw "$windowsBinary skills list failed with exit code $LASTEXITCODE"
}
$skills = $skillsJson | ConvertFrom-Json
$onesearchSkill = @($skills.skills | Where-Object { $_.id -eq "onesearch" })
$retiredSkillName = @($skills.skills | Where-Object { $_.id -eq "onesearch-cli" -or "onesearch-cli" -in @($_.aliases) })
if ($skills.ok -ne $true -or $onesearchSkill.Count -eq 0 -or $retiredSkillName.Count -gt 0) {
    throw "Bundled skill list does not include onesearch"
}

$skillContent = & $windowsBinary skills show onesearch --format content
if ($LASTEXITCODE -ne 0) {
    throw "$windowsBinary skills show onesearch failed with exit code $LASTEXITCODE"
}
$skillText = $skillContent -join [Environment]::NewLine
if ($skillText -notmatch '(?m)^# Onesearch Router$') {
    throw "Bundled onesearch router content is missing the expected heading"
}

$contractContent = & $windowsBinary skills show onesearch --file references/agent-execution-contract.md --format content
if ($LASTEXITCODE -ne 0) {
    throw "$windowsBinary skills show onesearch agent execution contract failed with exit code $LASTEXITCODE"
}
$contractText = $contractContent -join [Environment]::NewLine
if ($contractText -notmatch '(?m)^# Onesearch Agent Execution Contract$') {
    throw "Bundled agent execution contract content is missing the expected heading"
}

$packOrder = @(
    "npm\packages\win32-x64",
    "npm\packages\linux-x64",
    "npm\packages\darwin-arm64",
    "npm\onesearch",
    "npm\deqiying-onesearch"
)

try {
    foreach ($packageDir in $packOrder) {
        Invoke-NpmPack -PackageDir $packageDir
    }
}
finally {
    $env:npm_config_cache = $oldNpmCache
}
