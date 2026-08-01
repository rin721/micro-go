param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._~-]*(/[A-Za-z0-9][A-Za-z0-9._~-]*)+$')]
    [string]$ModulePath,

    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[a-z][a-z0-9-]*$')]
    [string]$ApplicationName,

    [string]$TargetDirectory = ".",

    [switch]$SkipVerify
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot/invoke-native.ps1"
$templateModule = "github.com/rin721/micro-go"
$templateName = "micro-go"
$target = (Resolve-Path -LiteralPath $TargetDirectory -ErrorAction Stop).Path
$volumeRoot = [IO.Path]::GetPathRoot($target)
if ($target -eq $volumeRoot) {
    throw "TargetDirectory must not be a volume root"
}
$goModPath = Join-Path $target "go.mod"
$agentRulesPath = Join-Path $target "AGENTS.md"
if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf) -or -not (Test-Path -LiteralPath $agentRulesPath -PathType Leaf)) {
    throw "TargetDirectory is not a micro-go project copy: $target"
}

$goMod = [IO.File]::ReadAllText($goModPath)
$moduleMatch = [regex]::Match($goMod, '(?m)^module\s+(\S+)\s*$')
if (-not $moduleMatch.Success) {
    throw "go.mod does not declare a module"
}
$currentModule = $moduleMatch.Groups[1].Value
if ($currentModule -ne $templateModule -and $currentModule -ne $ModulePath) {
    throw "refusing to replace unexpected module path: $currentModule"
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
$allowedExtensions = @(".go", ".md", ".yaml", ".yml", ".mod", ".ps1", ".sh")
$files = Get-ChildItem -LiteralPath $target -Recurse -File | Where-Object {
    $relative = $_.FullName.Substring($target.Length).TrimStart([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $segments = $relative -split '[\\/]'
    $isInitializer = $relative -eq "scripts\init-project.ps1" -or $relative -eq "scripts\init-project.sh"
    -not $isInitializer -and $segments -notcontains ".git" -and $segments -notcontains "tmp" -and $allowedExtensions -contains $_.Extension
}

foreach ($file in $files) {
    $content = [IO.File]::ReadAllText($file.FullName)
    $updated = $content.Replace($templateModule, $ModulePath)
    if ($file.Extension -eq ".md") {
        $updated = $updated.Replace($templateName, $ApplicationName)
    }
    if ($file.FullName -eq (Join-Path $target "internal\bootstrap\bootstrap.go")) {
        $updated = $updated.Replace('defaultApplicationName = "micro-go"', 'defaultApplicationName = "' + $ApplicationName + '"')
    }
    if ($file.FullName -eq (Join-Path $target "config\app.yaml")) {
        $updated = [regex]::Replace($updated, '(?m)^(\s*name:\s*)micro-go\s*$', '${1}' + $ApplicationName)
    }
    if ($updated -ne $content) {
        [IO.File]::WriteAllText($file.FullName, $updated, $utf8)
    }
}

$residual = Get-ChildItem -LiteralPath $target -Recurse -File -Filter *.go | Where-Object {
    $_.FullName -notmatch '[\\/]\.git[\\/]' -and [IO.File]::ReadAllText($_.FullName).Contains($templateModule)
}
if ($residual) {
    throw "template module path remains in Go source: $($residual.FullName -join ', ')"
}

if (-not $SkipVerify) {
    Push-Location $target
    try {
        Invoke-NativeCommand -Command { go mod tidy } -Description "go mod tidy"
        Invoke-NativeCommand -Command { go build ./... } -Description "go build"
        Invoke-NativeCommand -Command { go test -count=1 ./... } -Description "go test"
        Invoke-NativeCommand -Command { go test -count=1 ./internal/bootstrap -run '^TestRunBuildsAndStopsApplication$' } -Description "Bootstrap startup test"
    }
    finally {
        Pop-Location
    }
}

Write-Output "Initialized $ApplicationName with module $ModulePath in $target"
