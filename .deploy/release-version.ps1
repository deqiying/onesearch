$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$versionFile = Join-Path $PSScriptRoot "version"
$versionPattern = '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'

if (-not (Test-Path -LiteralPath $versionFile)) {
    throw "Missing .deploy/version"
}

$Version = (Get-Content -LiteralPath $versionFile -Raw).Trim()
if ($Version -notmatch $versionPattern) {
    throw ".deploy/version must contain a semver value like 0.1.1"
}

$tagName = "v$Version"

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

function Get-CheckedOutput {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$Arguments
    )

    $output = & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath failed with exit code $LASTEXITCODE"
    }
    return $output
}

function Write-Utf8NoBom {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Content
    )

    $encoding = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $Content, $encoding)
}

function Get-ReleaseCommitMessage {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Version
    )

    # Keep this script ASCII-only here: Windows PowerShell 5.1 can read
    # UTF-8-without-BOM script literals as ANSI and mojibake the commit title.
    $releasePrefix = [string]::Concat([char]0x53D1, [char]0x5E03)
    return "$releasePrefix $Version"
}

function Invoke-GitCommitWithUtf8Message {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Message
    )

    $messageFile = [System.IO.Path]::GetTempFileName()
    try {
        Write-Utf8NoBom -Path $messageFile -Content ($Message + [Environment]::NewLine)
        $gitCommitArgs = @("commit", "-F", $messageFile)
        Invoke-Checked git @gitCommitArgs
    }
    finally {
        Remove-Item -LiteralPath $messageFile -Force -ErrorAction SilentlyContinue
    }
}

function Update-PackageVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $package = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    $package.version = $Version

    if ($Path -like "*npm\onesearch\package.json") {
        $package.optionalDependencies.'@deqiying/onesearch-darwin-arm64' = $Version
        $package.optionalDependencies.'@deqiying/onesearch-linux-x64' = $Version
        $package.optionalDependencies.'@deqiying/onesearch-win32-x64' = $Version
    }

    if ($Path -like "*npm\deqiying-onesearch\package.json") {
        $package.dependencies.onesearch = $Version
    }

    return ($package | ConvertTo-Json -Depth 20) + [Environment]::NewLine
}

Push-Location $repoRoot
try {
    $versionFiles = @(
        ".deploy/version",
        "npm/packages/win32-x64/package.json",
        "npm/packages/linux-x64/package.json",
        "npm/packages/darwin-arm64/package.json",
        "npm/onesearch/package.json",
        "npm/deqiying-onesearch/package.json"
    )

    $allowedDirtyPaths = $versionFiles
    $statusLines = @(Get-CheckedOutput git status --porcelain --untracked-files=all)
    $dirtyPaths = @()
    foreach ($line in $statusLines) {
        if ($line.Length -lt 4) {
            continue
        }

        $path = $line.Substring(3).Trim('"') -replace '\\', '/'
        if ($path -like '* -> *') {
            $path = ($path -split ' -> ')[-1]
        }
        $dirtyPaths += $path
    }

    $unexpectedDirtyPaths = @($dirtyPaths | Where-Object { $allowedDirtyPaths -notcontains $_ })
    if ($unexpectedDirtyPaths.Count -gt 0) {
        $details = ($unexpectedDirtyPaths | ForEach-Object { "  $_" }) -join [Environment]::NewLine
        throw "Working tree has unrelated changes. Commit or stash them before preparing a release.$([Environment]::NewLine)$details"
    }

    & git rev-parse -q --verify "refs/tags/$tagName" *> $null
    if ($LASTEXITCODE -eq 0) {
        throw "Tag $tagName already exists."
    }

    $updates = @{}
    foreach ($file in $versionFiles) {
        if ($file -eq ".deploy/version") {
            continue
        }

        $fullPath = Join-Path $repoRoot $file
        $updates[$fullPath] = Update-PackageVersion -Path $fullPath
    }

    foreach ($entry in $updates.GetEnumerator()) {
        Write-Utf8NoBom -Path $entry.Key -Content $entry.Value
    }

    $gitAddArgs = @("add", "--") + $versionFiles
    Invoke-Checked git @gitAddArgs

    $gitDiffArgs = @("diff", "--cached", "--quiet", "--") + $versionFiles
    & git @gitDiffArgs
    if ($LASTEXITCODE -gt 1) {
        throw "git diff failed with exit code $LASTEXITCODE"
    }
    $hasStagedChanges = $LASTEXITCODE -eq 1

    if ($hasStagedChanges) {
        Invoke-GitCommitWithUtf8Message -Message (Get-ReleaseCommitMessage -Version $Version)
    }
    else {
        Write-Host "No version file changes detected; tagging current HEAD."
    }

    Invoke-Checked git tag $tagName

    $head = Get-CheckedOutput git rev-parse --short HEAD
    Write-Host "Prepared release $Version at commit $head with local tag $tagName."
    Write-Host "This script does not push to remote."
    Write-Host "Push later with:"
    Write-Host "  git push origin main"
    Write-Host "  git push origin $tagName"
    Write-Host "Pushing the tag triggers the Release workflow:"
    Write-Host "  - GitHub Release binaries are built with linker-injected version metadata."
    Write-Host "  - npm packages are built and published in parallel after shared tests."
}
finally {
    Pop-Location
}
