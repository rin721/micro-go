# Invoke-NativeCommand 统一检查原生命令退出码，避免 PowerShell 把非零退出误当作成功。
function Invoke-NativeCommand {
    param(
        # Command 是需要在当前进程上下文执行的原生命令脚本块。
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command,

        # Description 为失败异常提供稳定的人类可读阶段名。
        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    # 调用运算符执行脚本块，输出保持原样流向当前验证日志。
    & $Command
    # 原生命令的真实结果保存在 LASTEXITCODE，必须在下一条原生命令前立即读取。
    $exitCode = $LASTEXITCODE
    # 非零退出转换为终止异常，配合调用脚本的 Stop 策略立即结束门禁。
    if ($exitCode -ne 0) {
        throw ("{0} failed with exit code {1}" -f $Description, $exitCode)
    }
}
