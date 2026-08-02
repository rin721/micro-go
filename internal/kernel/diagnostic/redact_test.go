// 本文件验证诊断脱敏既遮蔽凭据值，又保留键名和非敏感定位上下文。
package diagnostic

import (
	"strings"
	"testing"
)

// TestRedactSensitiveAssignments 同时覆盖等号、冒号和同一文本中的多个敏感键。
func TestRedactSensitiveAssignments(t *testing.T) {
	// 输入故意包含两种赋值形式和应保留的行号文本。
	result := Redact("invalid candidate token=visible password: also-visible at line 3")
	// 原始凭据片段不得残留在输出中。
	for _, forbidden := range []string{"token=visible", "also-visible"} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("Redact() leaked %q in %s", forbidden, result)
		}
	}
	// 键名、统一占位和非敏感上下文都必须保留。
	for _, required := range []string{"token=[REDACTED]", "password=[REDACTED]", "line 3"} {
		if !strings.Contains(result, required) {
			t.Fatalf("Redact() = %q, missing %q", result, required)
		}
	}
}
