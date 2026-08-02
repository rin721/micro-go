// 本文件在诊断文本离开 Kernel 前集中遮蔽常见凭据赋值。
package diagnostic

import (
	"regexp"
	"strings"
)

// sensitiveAssignment 匹配常见敏感键及其紧邻值，不扫描或保存完整配置对象。
var sensitiveAssignment = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization|dsn)\b\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;]+)`)

// Redact 隐去错误摘要中的常见敏感赋值，同时保留键名和其余诊断上下文。
func Redact(message string) string {
	// 每个命中项单独替换，非敏感上下文和多个键之间的文本保持原样。
	return sensitiveAssignment.ReplaceAllStringFunc(message, func(value string) string {
		// 正则已经约束分隔符，但仍防御异常匹配，避免切片越界。
		separator := strings.IndexAny(value, ":=")
		if separator < 0 {
			return "[REDACTED]"
		}
		// 保留原键名以支持定位，只把值统一替换为不可逆占位文本。
		return strings.TrimSpace(value[:separator]) + "=[REDACTED]"
	})
}
