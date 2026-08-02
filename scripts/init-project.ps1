# 本脚本在已复制的目标目录内替换模板 Module 和应用身份，不生成业务代码。
# ModulePath 必须是至少包含一个斜杠的合法 Go Module 风格路径。
# ApplicationName 同时用于文档模板名、默认配置和 Bootstrap 默认身份。
# TargetDirectory 默认为当前目录，但会在任何写入前解析并验证边界。
# SkipVerify 只跳过目标副本中的构建测试，不跳过身份残留扫描。
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

# 任何文件系统、正则或原生命令异常都立即终止，禁止继续部分替换。
$ErrorActionPreference = "Stop"
# 导入统一原生命令退出码检查函数。
. "$PSScriptRoot/invoke-native.ps1"
# 两个模板常量分别控制 Go import 身份和 Markdown/应用显示名。
$templateModule = "github.com/rin721/micro-go"
$templateName = "micro-go"
# 解析为存在的绝对路径，后续所有读写都限制在该目标下。
$target = (Resolve-Path -LiteralPath $TargetDirectory -ErrorAction Stop).Path
# 卷根是过宽且危险的替换目标，必须显式拒绝。
$volumeRoot = [IO.Path]::GetPathRoot($target)
if ($target -eq $volumeRoot) {
    throw "TargetDirectory must not be a volume root"
}
# go.mod 和 AGENTS.md 共同证明目标是完整 micro-go 项目副本，而不是任意目录。
$goModPath = Join-Path $target "go.mod"
$agentRulesPath = Join-Path $target "AGENTS.md"
if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf) -or -not (Test-Path -LiteralPath $agentRulesPath -PathType Leaf)) {
    throw "TargetDirectory is not a micro-go project copy: $target"
}

# 读取当前 Module 声明，既允许首次初始化，也允许相同参数安全重跑。
$goMod = [IO.File]::ReadAllText($goModPath)
$moduleMatch = [regex]::Match($goMod, '(?m)^module\s+(\S+)\s*$')
if (-not $moduleMatch.Success) {
    throw "go.mod does not declare a module"
}
$currentModule = $moduleMatch.Groups[1].Value
# 非模板且不等于目标身份说明目录已经属于另一项目，拒绝破坏性替换。
if ($currentModule -ne $templateModule -and $currentModule -ne $ModulePath) {
    throw "refusing to replace unexpected module path: $currentModule"
}

# 所有写回文件统一使用无 BOM UTF-8，保持跨平台工具兼容。
$utf8 = New-Object System.Text.UTF8Encoding($false)
# 只扫描明确的文本类型，避免损坏二进制或 Git 内部对象。
$allowedExtensions = @(".go", ".md", ".yaml", ".yml", ".mod", ".ps1", ".sh")
# 排除 .git、tmp 和两个初始化脚本自身，防止替换后失去模板识别能力。
$files = Get-ChildItem -LiteralPath $target -Recurse -File | Where-Object {
    $relative = $_.FullName.Substring($target.Length).TrimStart([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $segments = $relative -split '[\\/]'
    $isInitializer = $relative -eq "scripts\init-project.ps1" -or $relative -eq "scripts\init-project.sh"
    -not $isInitializer -and $segments -notcontains ".git" -and $segments -notcontains "tmp" -and $allowedExtensions -contains $_.Extension
}

# 每个候选文件先在内存完成全部替换，内容确实变化时才原子式写回该文件。
foreach ($file in $files) {
    $content = [IO.File]::ReadAllText($file.FullName)
    $updated = $content.Replace($templateModule, $ModulePath)
    # Markdown 除 Module path 外还把模板显示名替换为新应用名。
    if ($file.Extension -eq ".md") {
        $updated = $updated.Replace($templateName, $ApplicationName)
    }
    # Bootstrap 常量使用精确文本替换，避免误改其他 micro-go 语义文本。
    if ($file.FullName -eq (Join-Path $target "internal\bootstrap\bootstrap.go")) {
        $updated = $updated.Replace('defaultApplicationName = "micro-go"', 'defaultApplicationName = "' + $ApplicationName + '"')
    }
    # 默认 YAML 只替换 application.name 行，不修改日志等其他值。
    if ($file.FullName -eq (Join-Path $target "config\app.yaml")) {
        $updated = [regex]::Replace($updated, '(?m)^(\s*name:\s*)micro-go\s*$', '${1}' + $ApplicationName)
    }
    # 未变化文件不写回，从而保留时间戳并减少无意义 Diff。
    if ($updated -ne $content) {
        [IO.File]::WriteAllText($file.FullName, $updated, $utf8)
    }
}

# 替换结束后重新扫描全部 Go 文件，任何旧 Module import 残留都阻止成功退出。
$residual = Get-ChildItem -LiteralPath $target -Recurse -File -Filter *.go | Where-Object {
    $_.FullName -notmatch '[\\/]\.git[\\/]' -and [IO.File]::ReadAllText($_.FullName).Contains($templateModule)
}
if ($residual) {
    throw "template module path remains in Go source: $($residual.FullName -join ', ')"
}

# 默认在目标副本内执行依赖整理、完整构建测试和真实 Bootstrap 启停测试。
if (-not $SkipVerify) {
    # Push/Pop 放在 try/finally 中，失败也恢复调用者原工作目录。
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

# 仅在替换和所有启用验证均成功后输出完成摘要。
Write-Output "Initialized $ApplicationName with module $ModulePath in $target"
