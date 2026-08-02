# 完整入口执行当前地基的 unit/static 门禁.
# 任一子脚本异常都终止当前入口并向调用方返回失败。
$ErrorActionPreference = "Stop"

# 使用 PSScriptRoot 定位同目录脚本，调用结果不依赖当前工作目录的路径表示。
& "$PSScriptRoot/verify-unit.ps1"
